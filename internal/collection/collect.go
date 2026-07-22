package collection

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/audit"
	"github.com/forecastiq/forecastiq/internal/catalog"
	"github.com/forecastiq/forecastiq/internal/collection/domain"
	"github.com/forecastiq/forecastiq/internal/collection/ports"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
	"github.com/forecastiq/forecastiq/internal/platform/events"
	"github.com/forecastiq/forecastiq/internal/platform/ids"
	"github.com/forecastiq/forecastiq/internal/platform/metrics"
)

// CollectService implements ForecastCollector: the complete, observable,
// idempotent forecast-collection workflow (docs/workflows/01).
type CollectService struct {
	adapters    map[string]ports.ForecastProviderAdapter
	collections ports.CollectionRepository
	snapshots   ports.SnapshotRepository
	payloads    ports.PayloadStore
	circuits    catalog.CircuitState
	bus         events.Bus
	metrics     *metrics.Metrics
	audit       audit.Recorder
	clock       clock.Clock
	logger      *slog.Logger
	tx          *dbtx.Runner
	pool        dbtx.DBTX
	resolveCred func(credentialRef string) string
}

// NewCollectService wires a CollectService. adapters is keyed by provider slug.
func NewCollectService(
	adapters map[string]ports.ForecastProviderAdapter,
	collections ports.CollectionRepository,
	snapshots ports.SnapshotRepository,
	payloads ports.PayloadStore,
	circuits catalog.CircuitState,
	bus events.Bus,
	m *metrics.Metrics,
	rec audit.Recorder,
	clk clock.Clock,
	logger *slog.Logger,
	tx *dbtx.Runner,
	pool dbtx.DBTX,
	resolveCred func(string) string,
) *CollectService {
	return &CollectService{
		adapters: adapters, collections: collections, snapshots: snapshots,
		payloads: payloads, circuits: circuits, bus: bus, metrics: m, audit: rec,
		clock: clk, logger: logger, tx: tx, pool: pool, resolveCred: resolveCred,
	}
}

// Collect runs one forecast collection end-to-end.
func (s *CollectService) Collect(ctx context.Context, in CollectInput) (*domain.ForecastCollection, error) {
	provider, location, config := in.Provider, in.Location, in.Config
	now := s.clock.Now()
	log := s.logger.With(
		slog.String("provider", provider.Slug),
		slog.String("location_id", location.ID.String()))

	if !provider.Status.Active() || !location.Status.Active() || !config.Status.Active() {
		return nil, domain.ErrInactive
	}
	adapter, ok := s.adapters[provider.Slug]
	if !ok {
		return nil, fmt.Errorf("provider %q: %w", provider.Slug, domain.ErrInactive)
	}

	// 1. Circuit pre-check (FC-09).
	decision, err := s.circuits.Evaluate(ctx, provider.ID, now)
	if err != nil {
		return nil, fmt.Errorf("circuit evaluate: %w", err)
	}
	s.metrics.CircuitState.WithLabelValues(provider.Slug).Set(decision.State.GaugeValue())
	if !decision.Allowed {
		log.WarnContext(ctx, "collection.circuit_open", slog.Any("retry_at", decision.RetryAt))
		return nil, &domain.CircuitOpenError{ProviderID: provider.ID, RetryAt: decision.RetryAt}
	}

	// 2. Deterministic issuance — the collection-level dedup key when the
	// provider exposes no model-run time. Set one hour before the first
	// predicted period so every hourly period satisfies target_time > issued_at
	// and horizons are clean multiples of 60 minutes (Open-Meteo exposes no
	// model-run time; documented adapter convention).
	issuedAt := now.UTC().Truncate(time.Hour).Add(-time.Hour)

	// 3. Provider call (adapter owns retry + rate-limit + schema validation).
	log.InfoContext(ctx, "collection.started", slog.String("source", in.Source))
	collectStart := s.clock.Now()
	result, err := adapter.FetchForecast(ctx, ports.ForecastRequest{
		ProviderID:   provider.ID,
		LocationID:   location.ID,
		ProviderSlug: provider.Slug,
		BaseURL:      provider.APIBaseURL,
		Credential:   s.resolveCred(config.CredentialRef),
		Latitude:     location.Latitude,
		Longitude:    location.Longitude,
		Timezone:     location.Timezone,
		IssuedAt:     issuedAt,
	})
	s.metrics.CollectionDuration.WithLabelValues(provider.Slug).Observe(s.clock.Now().Sub(collectStart).Seconds())
	if err != nil {
		return nil, fmt.Errorf("provider fetch: %w", err)
	}
	s.observeProvider(provider.Slug, result)

	collectionID := ids.New()

	// 4. Collection-level deduplication (domain §4.3) for usable outcomes.
	dedupKey := issuedAt
	if result.ModelRunTime != nil {
		dedupKey = *result.ModelRunTime
	}
	if result.Outcome == ports.OutcomeSuccess || result.Outcome == ports.OutcomePartial {
		existing, ferr := s.collections.FindDedup(ctx, s.pool, provider.ID, location.ID, dedupKey)
		if ferr != nil {
			return nil, fmt.Errorf("dedup lookup: %w", ferr)
		}
		if existing != nil {
			return s.storeDeduplicated(ctx, in, collectionID, issuedAt, result, existing, log)
		}
	}

	// 5. Persist raw payload (degrade gracefully on failure; FC payload step).
	objectKey := ports.BuildPayloadKey(provider.Slug, now, collectionID)
	payloadOK := true
	if len(result.RawPayload) > 0 {
		if werr := s.payloads.Write(ctx, objectKey, result.RawPayload); werr != nil {
			payloadOK = false
			log.ErrorContext(ctx, "payload.write_failed", slog.String("object_key", objectKey), slog.String("error", werr.Error()))
		}
	}

	status, errorCode, errorMsg := classify(result, payloadOK)
	completedAt := s.clock.Now()

	// 6. Ensure target-month partitions exist (idempotent DDL) before insert.
	if status.Successful() && len(result.Snapshots) > 0 {
		if perr := s.snapshots.EnsurePartitions(ctx, s.pool, monthStarts(result.Snapshots)); perr != nil {
			return nil, fmt.Errorf("ensure partitions: %w", perr)
		}
	}

	// 7. Single bounded transaction: collection + snapshots + circuit (ADR-027).
	coll := &domain.ForecastCollection{
		ID:                      collectionID,
		ProviderID:              provider.ID,
		LocationID:              location.ID,
		ProviderConfigurationID: config.ID,
		RequestedAt:             issuedAt,
		Status:                  domain.StatusPending,
		ProviderRequestID:       result.ProviderRequestID,
		ProviderModelRunTime:    result.ModelRunTime,
		RawPayloadChecksum:      result.Checksum,
		ResponseStatusCode:      result.HTTPStatusCode,
		ResponseLatencyMS:       result.LatencyMS,
		SchemaVersion:           result.SchemaVersion,
		AdapterVersion:          result.AdapterVersion,
		CreatedAt:               now,
	}
	if payloadOK && len(result.RawPayload) > 0 {
		coll.RawPayloadObjectKey = objectKey
	}

	var storedCount int
	var transition catalog.Transition
	txErr := s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		if ierr := s.collections.Insert(ctx, tx, coll); ierr != nil {
			return ierr
		}
		if status.Successful() && len(result.Snapshots) > 0 {
			for _, snap := range result.Snapshots {
				snap.ForecastCollectionID = coll.ID
				snap.ProviderID = provider.ID
				snap.LocationID = location.ID
			}
			stored, berr := s.snapshots.InsertBatch(ctx, tx, result.Snapshots)
			if berr != nil {
				return berr
			}
			storedCount = stored
		}
		coll.SnapshotsStored = storedCount
		coll.SnapshotsDeduplicated = len(result.Snapshots) - storedCount
		coll.SnapshotsInvalid = result.InvalidCount
		coll.RecordsReceived = result.RecordsReceived
		coll.Status = status
		coll.CompletedAt = &completedAt
		coll.ErrorCode = errorCode
		coll.ErrorMessage = errorMsg
		if uerr := s.collections.Complete(ctx, tx, coll); uerr != nil {
			return uerr
		}
		// Circuit outcome: success/partial closes; failed/timeout advances;
		// rate_limited/auth_failed leave the breaker unchanged.
		switch {
		case status.Successful():
			transition, _ = s.circuits.RecordSuccess(ctx, tx, provider.ID, completedAt)
		case result.Outcome == ports.OutcomeFailed || result.Outcome == ports.OutcomeTimeout:
			transition, _ = s.circuits.RecordFailure(ctx, tx, provider.ID, completedAt)
		}
		return nil
	})
	if txErr != nil {
		if errors.Is(txErr, domain.ErrDuplicateCollection) {
			// A concurrent collection committed the same dedup key first; record
			// this attempt as deduplicated (domain §4.3) rather than failing.
			existing, _ := s.collections.FindDedup(ctx, s.pool, in.Provider.ID, in.Location.ID, dedupKey)
			if existing == nil {
				existing = &domain.ForecastCollection{}
			}
			return s.storeDeduplicated(ctx, in, collectionID, issuedAt, result, existing, log)
		}
		return nil, fmt.Errorf("persist collection: %w", txErr)
	}

	// 8. Post-commit observability: metrics + events.
	s.postCommit(ctx, provider, coll, storedCount, transition, log)

	// 9. Audit (manual trigger / replay only; scheduled collections are
	// recorded in schedule_runs + the collection row itself).
	if in.Source == SourceManual || in.Source == SourceReplay {
		_ = s.audit.Record(ctx, s.pool, audit.Event{
			UserID:       in.Actor.UserID,
			Action:       "collection." + in.Source,
			ResourceType: "forecast_collection",
			ResourceID:   &coll.ID,
			IPAddress:    in.Actor.IPAddress,
			Details: map[string]any{
				"actor":       in.Actor.Name,
				"provider":    provider.Slug,
				"location_id": location.ID.String(),
				"status":      string(coll.Status),
			},
		})
	}

	return coll, nil
}

// storeDeduplicated records a deduplicated collection (zero new snapshots, no
// payload rewrite; domain §4.3) and emits the event.
func (s *CollectService) storeDeduplicated(ctx context.Context, in CollectInput, collectionID uuid.UUID,
	issuedAt time.Time, result *ports.ForecastResult, existing *domain.ForecastCollection, log *slog.Logger) (*domain.ForecastCollection, error) {
	now := s.clock.Now()
	completedAt := now
	coll := &domain.ForecastCollection{
		ID:                      collectionID,
		ProviderID:              in.Provider.ID,
		LocationID:              in.Location.ID,
		ProviderConfigurationID: in.Config.ID,
		RequestedAt:             issuedAt,
		CompletedAt:             &completedAt,
		Status:                  domain.StatusDeduplicated,
		ProviderModelRunTime:    result.ModelRunTime,
		RecordsReceived:         result.RecordsReceived,
		SnapshotsDeduplicated:   result.RecordsReceived,
		SchemaVersion:           result.SchemaVersion,
		AdapterVersion:          result.AdapterVersion,
		CreatedAt:               now,
	}
	if err := s.collections.Insert(ctx, s.pool, coll); err != nil {
		return nil, fmt.Errorf("persist deduplicated collection: %w", err)
	}
	s.metrics.CollectionAttempts.WithLabelValues(in.Provider.Slug, string(domain.StatusDeduplicated)).Inc()
	s.bus.Publish(ctx, events.ForecastCollected{
		CollectionID: coll.ID, ProviderID: in.Provider.ID, LocationID: in.Location.ID,
		SnapshotCount: 0, Status: string(domain.StatusDeduplicated), At: now,
	})
	log.InfoContext(ctx, "collection.deduplicated",
		slog.String("collection_id", coll.ID.String()),
		slog.String("existing_collection_id", existing.ID.String()))
	return coll, nil
}

func (s *CollectService) observeProvider(slug string, result *ports.ForecastResult) {
	s.metrics.ProviderLatency.WithLabelValues(slug).Observe(float64(result.LatencyMS) / 1000.0)
	if result.Outcome == ports.OutcomeRateLimited {
		s.metrics.RateLimitHits.WithLabelValues(slug).Inc()
	}
	for code, n := range result.UnmappedConditions {
		s.metrics.ConditionUnmapped.WithLabelValues(slug, code).Add(float64(n))
	}
}

func (s *CollectService) postCommit(ctx context.Context, provider *catalog.Provider, coll *domain.ForecastCollection,
	storedCount int, transition catalog.Transition, log *slog.Logger) {
	slug := provider.Slug
	s.metrics.CollectionAttempts.WithLabelValues(slug, string(coll.Status)).Inc()
	if storedCount > 0 {
		s.metrics.SnapshotsStored.WithLabelValues(slug).Add(float64(storedCount))
	}
	if coll.SnapshotsInvalid > 0 {
		reason := "invalid_range"
		if coll.ErrorCode == "schema_drift" {
			reason = "schema"
		}
		s.metrics.RecordsRejected.WithLabelValues(slug, reason).Add(float64(coll.SnapshotsInvalid))
	}
	if transition.Changed {
		s.metrics.CircuitState.WithLabelValues(slug).Set(transition.New.GaugeValue())
		s.bus.Publish(ctx, events.ProviderHealthChanged{
			ProviderID: transition.ProviderID, OldState: string(transition.Old),
			NewState: string(transition.New), ConsecutiveFailures: transition.ConsecutiveFailures,
		})
	}
	s.bus.Publish(ctx, events.ForecastCollected{
		CollectionID: coll.ID, ProviderID: provider.ID, LocationID: coll.LocationID,
		SnapshotCount: storedCount, Status: string(coll.Status), At: s.clock.Now(),
	})
	log.InfoContext(ctx, "collection.completed",
		slog.String("collection_id", coll.ID.String()),
		slog.String("status", string(coll.Status)),
		slog.Int("records_received", coll.RecordsReceived),
		slog.Int("snapshots_stored", coll.SnapshotsStored),
		slog.Int("snapshots_deduplicated", coll.SnapshotsDeduplicated),
		slog.Int("snapshots_invalid", coll.SnapshotsInvalid),
		slog.String("error_code", coll.ErrorCode))
}

// classify maps an adapter outcome to a collection status + FC-13 error code.
func classify(result *ports.ForecastResult, payloadOK bool) (domain.CollectionStatus, string, string) {
	var errorCode, errorMsg string
	if len(result.InvalidReasons) > 0 {
		errorMsg = strings.Join(truncate(result.InvalidReasons, 5), "; ")
	}
	if result.Err != nil && errorMsg == "" {
		errorMsg = result.Err.Error()
	}
	if !payloadOK {
		errorCode = "payload_write_failed"
	}

	switch result.Outcome {
	case ports.OutcomeSuccess:
		return domain.StatusSuccess, errorCode, errorMsg
	case ports.OutcomePartial:
		if errorCode == "" {
			errorCode = "partial_validation"
		}
		return domain.StatusPartial, errorCode, errorMsg
	case ports.OutcomeRateLimited:
		return domain.StatusRateLimited, orDefault(errorCode, "rate_limited"), errorMsg
	case ports.OutcomeTimeout:
		return domain.StatusTimeout, orDefault(errorCode, "timeout"), errorMsg
	case ports.OutcomeAuthFailed:
		return domain.StatusFailed, orDefault(errorCode, "invalid_credentials"), errorMsg
	default: // OutcomeFailed
		return domain.StatusFailed, orDefault(errorCode, result.ErrorCode), errorMsg
	}
}

func orDefault(v, def string) string {
	if v != "" {
		return v
	}
	return def
}

func truncate(in []string, n int) []string {
	if len(in) <= n {
		return in
	}
	return in[:n]
}

// monthStarts returns the distinct first-of-month instants (UTC) covering the
// snapshots' target times — the partitions that must exist before insert.
func monthStarts(snapshots []*domain.ForecastSnapshot) []time.Time {
	seen := map[time.Time]struct{}{}
	var out []time.Time
	for _, snap := range snapshots {
		t := snap.TargetTime.UTC()
		ms := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
		if _, ok := seen[ms]; !ok {
			seen[ms] = struct{}{}
			out = append(out, ms)
		}
	}
	return out
}
