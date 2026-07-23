package collection

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/audit"
	"github.com/forecastiq/forecastiq/internal/catalog"
	"github.com/forecastiq/forecastiq/internal/collection/domain"
	"github.com/forecastiq/forecastiq/internal/collection/ports"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
	"github.com/forecastiq/forecastiq/internal/platform/events"
	"github.com/forecastiq/forecastiq/internal/platform/ids"
)

// Replay reprocesses the stored raw payload of an existing collection through
// the current adapter (FC-14; workflow 06 §2). It never touches the provider
// network and never mutates the original collection or its snapshots: a new
// collection is written and snapshots are inserted with dedup, so only rows
// that differ from what is already stored land. Re-running a replay is
// idempotent (all snapshots already present → zero stored).
func (s *CollectService) Replay(ctx context.Context, collectionID uuid.UUID, actor catalog.Actor) (*domain.ForecastCollection, error) {
	orig, err := s.collections.GetByID(ctx, s.pool, collectionID)
	if err != nil {
		return nil, err // ErrNotFound → 404
	}
	log := s.logger.With(slog.String("original_collection_id", orig.ID.String()))

	// A payload key + checksum are prerequisites; without them there is
	// nothing to replay (expired beyond retention or never captured).
	if orig.RawPayloadObjectKey == "" || orig.RawPayloadChecksum == "" {
		return nil, domain.ErrPayloadUnavailable
	}

	provider, err := s.providers.GetProvider(ctx, orig.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("load provider: %w", err)
	}
	adapter, ok := s.adapters[provider.Slug]
	if !ok {
		return nil, fmt.Errorf("provider %q: %w", provider.Slug, domain.ErrInactive)
	}
	replayer, ok := adapter.(ports.ReplayDecoder)
	if !ok {
		return nil, domain.ErrReplayUnsupported
	}

	// Read + verify integrity. A read failure means the payload is gone or
	// corrupt; a checksum mismatch quarantines the bytes so they cannot be
	// replayed or served again (workflow 06 §2).
	raw, err := s.payloads.Read(ctx, orig.RawPayloadObjectKey)
	if err != nil {
		log.WarnContext(ctx, "replay.payload_read_failed", slog.String("error", err.Error()))
		return nil, domain.ErrPayloadUnavailable
	}
	if !ports.VerifyChecksum(raw, orig.RawPayloadChecksum) {
		quarantineKey, qerr := s.payloads.Quarantine(ctx, orig.RawPayloadObjectKey)
		log.ErrorContext(ctx, "replay.payload_checksum_mismatch",
			slog.String("quarantine_key", quarantineKey), slog.Any("quarantine_error", qerr))
		s.metrics.RecordsRejected.WithLabelValues(provider.Slug, "checksum_mismatch").Inc()
		return nil, domain.ErrPayloadUnavailable
	}

	// Decode with the CURRENT adapter, preserving the original issuance so
	// snapshot keys (provider, location, issued_at, target_time) align with
	// the originals and dedup correctly.
	result, err := replayer.DecodeStored(ctx, ports.ForecastRequest{
		ProviderID:   orig.ProviderID,
		LocationID:   orig.LocationID,
		ProviderSlug: provider.Slug,
		IssuedAt:     orig.RequestedAt,
	}, raw)
	if err != nil {
		return nil, fmt.Errorf("decode stored payload: %w", err)
	}

	now := s.clock.Now()
	status, errorCode, errorMsg := classify(result, true)
	newID := ids.New()

	// A distinct requested_at keeps the replay out of the original's
	// collection-level dedup key; model-run time is intentionally cleared so
	// the replay is never mistaken for a fresh provider model run.
	coll := &domain.ForecastCollection{
		ID:                      newID,
		ProviderID:              orig.ProviderID,
		LocationID:              orig.LocationID,
		ProviderConfigurationID: orig.ProviderConfigurationID,
		RequestedAt:             now,
		Status:                  domain.StatusPending,
		ProviderRequestID:       orig.ProviderRequestID,
		RawPayloadObjectKey:     orig.RawPayloadObjectKey,
		RawPayloadChecksum:      orig.RawPayloadChecksum,
		SchemaVersion:           result.SchemaVersion,
		AdapterVersion:          result.AdapterVersion,
		CreatedAt:               now,
	}

	if status.Successful() && len(result.Snapshots) > 0 {
		if perr := s.snapshots.EnsurePartitions(ctx, s.pool, monthStarts(result.Snapshots)); perr != nil {
			return nil, fmt.Errorf("ensure partitions: %w", perr)
		}
	}

	var storedCount int
	txErr := s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		if ierr := s.collections.Insert(ctx, tx, coll); ierr != nil {
			return ierr
		}
		if status.Successful() && len(result.Snapshots) > 0 {
			for _, snap := range result.Snapshots {
				snap.ForecastCollectionID = coll.ID
				snap.ProviderID = orig.ProviderID
				snap.LocationID = orig.LocationID
			}
			stored, berr := s.snapshots.InsertBatch(ctx, tx, result.Snapshots)
			if berr != nil {
				return berr
			}
			storedCount = stored
		}
		completedAt := s.clock.Now()
		coll.SnapshotsStored = storedCount
		coll.SnapshotsDeduplicated = len(result.Snapshots) - storedCount
		coll.SnapshotsInvalid = result.InvalidCount
		coll.RecordsReceived = result.RecordsReceived
		coll.Status = status
		coll.CompletedAt = &completedAt
		coll.ErrorCode = errorCode
		coll.ErrorMessage = errorMsg
		return s.collections.Complete(ctx, tx, coll)
	})
	if txErr != nil {
		return nil, fmt.Errorf("persist replay collection: %w", txErr)
	}

	s.metrics.CollectionAttempts.WithLabelValues(provider.Slug, string(coll.Status)).Inc()
	if storedCount > 0 {
		s.metrics.SnapshotsStored.WithLabelValues(provider.Slug).Add(float64(storedCount))
	}
	s.bus.Publish(ctx, events.ForecastCollected{
		CollectionID: coll.ID, ProviderID: orig.ProviderID, LocationID: orig.LocationID,
		SnapshotCount: storedCount, Status: string(coll.Status), At: now,
	})
	_ = s.audit.Record(ctx, s.pool, audit.Event{
		UserID:       actor.UserID,
		Action:       "collection." + SourceReplay,
		ResourceType: "forecast_collection",
		ResourceID:   &coll.ID,
		IPAddress:    actor.IPAddress,
		Details: map[string]any{
			"actor":                  actor.Name,
			"provider":               provider.Slug,
			"original_collection_id": orig.ID.String(),
			"snapshots_stored":       storedCount,
			"snapshots_deduplicated": coll.SnapshotsDeduplicated,
			"status":                 string(coll.Status),
		},
	})
	log.InfoContext(ctx, "collection.replayed",
		slog.String("collection_id", coll.ID.String()),
		slog.String("status", string(coll.Status)),
		slog.Int("snapshots_stored", storedCount),
		slog.Int("snapshots_deduplicated", coll.SnapshotsDeduplicated))
	return coll, nil
}
