package collection

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/catalog"
	"github.com/forecastiq/forecastiq/internal/collection/domain"
	"github.com/forecastiq/forecastiq/internal/collection/ports"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
	"github.com/forecastiq/forecastiq/internal/platform/events"
	"github.com/forecastiq/forecastiq/internal/platform/metrics"
)

// ObserveService runs one observation collection for a location-window
// (docs/workflows/02): fetch → range/provenance normalize → dedup + correction
// cascade in one transaction → events + metrics. Observations have no parent
// collection entity (ADR-025); collector health derives from observation
// aggregates (freshness gauge).
type ObserveService struct {
	adapter      ports.ObservationSourceAdapter
	observations ports.ObservationRepository
	bus          events.Bus
	metrics      *metrics.Metrics
	clock        clock.Clock
	logger       *slog.Logger
	tx           *dbtx.Runner
	pool         dbtx.DBTX
}

// NewObserveService wires an ObserveService.
func NewObserveService(
	adapter ports.ObservationSourceAdapter,
	observations ports.ObservationRepository,
	bus events.Bus,
	m *metrics.Metrics,
	clk clock.Clock,
	logger *slog.Logger,
	tx *dbtx.Runner,
	pool dbtx.DBTX,
) *ObserveService {
	return &ObserveService{
		adapter: adapter, observations: observations, bus: bus, metrics: m,
		clock: clk, logger: logger, tx: tx, pool: pool,
	}
}

// correction pairs an old (superseded) observation with its replacement.
type correction struct {
	oldID         uuid.UUID
	oldObservedAt time.Time
	newID         uuid.UUID
	observedAt    time.Time
}

// Observe collects observations for [windowStart, windowEnd] at a location. It
// returns the number of new rows stored (for scheduler run history). A
// classified source failure (timeout / 5xx / schema drift) returns an error so
// the slot retries; a successful call with zero rows (source gap) is not an
// error (workflow §6).
func (s *ObserveService) Observe(ctx context.Context, location *catalog.Location, windowStart, windowEnd time.Time) (int, error) {
	if !location.Status.Active() {
		return 0, domain.ErrInactive
	}
	source := s.adapter.Source()
	log := s.logger.With(slog.String("source", source), slog.String("location_id", location.ID.String()))

	result, err := s.adapter.FetchObservations(ctx, ports.ObservationRequest{
		LocationID:  location.ID,
		Source:      source,
		Latitude:    location.Latitude,
		Longitude:   location.Longitude,
		Timezone:    location.Timezone,
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
	})
	if err != nil {
		return 0, fmt.Errorf("observation fetch: %w", err)
	}
	if result.Outcome != ports.OutcomeSuccess {
		return 0, fmt.Errorf("observation fetch %s: %s", result.Outcome, orDefault(result.ErrorCode, "error"))
	}

	var stored int
	var corrections []correction
	var storedObs []*domain.Observation
	txErr := s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		if perr := s.observations.EnsurePartitions(ctx, tx, observationMonthStarts(result.Observations)); perr != nil {
			return perr
		}
		existing, lerr := s.observations.ListCurrentByWindow(ctx, tx, source, location.ID, windowStart, windowEnd)
		if lerr != nil {
			return lerr
		}
		byHour := make(map[time.Time]*domain.Observation, len(existing))
		for _, o := range existing {
			byHour[o.ObservedAt.UTC()] = o
		}

		var toInsert []*domain.Observation
		corrections = corrections[:0]
		for _, obs := range result.Observations {
			prev, ok := byHour[obs.ObservedAt.UTC()]
			switch {
			case !ok:
				toInsert = append(toInsert, obs) // new hour
			case obs.DiffersFrom(prev):
				// Correction (workflow §4): the new row is flagged corrected;
				// the old row is superseded (must happen before the insert so
				// the live-row dedup index does not conflict).
				obs.QualityFlag = domain.QualityCorrected
				corrections = append(corrections, correction{
					oldID: prev.ID, oldObservedAt: prev.ObservedAt, newID: obs.ID, observedAt: obs.ObservedAt.UTC(),
				})
				toInsert = append(toInsert, obs)
			default:
				// Values equal within ε → deduplicated (no-op).
			}
		}
		for _, c := range corrections {
			if serr := s.observations.Supersede(ctx, tx, c.oldID, c.oldObservedAt, c.newID); serr != nil {
				return serr
			}
		}
		n, ierr := s.observations.InsertBatch(ctx, tx, toInsert)
		if ierr != nil {
			return ierr
		}
		stored = n
		storedObs = toInsert
		return nil
	})
	if txErr != nil {
		return 0, fmt.Errorf("persist observations: %w", txErr)
	}

	s.postCommit(ctx, source, location, result, storedObs, stored, corrections, log)
	return stored, nil
}

// postCommit emits metrics + events after the storage transaction commits.
func (s *ObserveService) postCommit(ctx context.Context, source string, location *catalog.Location,
	result *ports.ObservationResult, storedObs []*domain.Observation, stored int, corrections []correction, log *slog.Logger) {
	loc := location.ID.String()
	typeMix := map[string]int{}
	suspectStored := 0
	for _, o := range storedObs {
		typeMix[string(o.ObservationType)]++
		if o.QualityFlag == domain.QualitySuspect {
			suspectStored++
		}
	}
	if stored > 0 {
		s.metrics.ObservationsCollected.WithLabelValues(source, loc).Add(float64(stored))
	}
	if suspectStored > 0 {
		s.metrics.ObservationsSuspect.WithLabelValues(source, "range").Add(float64(suspectStored))
	}
	// Freshness gauge: age of the newest observation for this location.
	if latest := newestObservedAt(result.Observations); !latest.IsZero() {
		age := s.clock.Now().UTC().Sub(latest).Seconds()
		if age < 0 {
			age = 0
		}
		s.metrics.ObservationFreshness.WithLabelValues(loc).Set(age)
	}

	s.bus.Publish(ctx, events.ObservationCollected{
		LocationID: location.ID, Source: source, Count: stored, ObservationTypeMix: typeMix,
	})
	for _, c := range corrections {
		s.bus.Publish(ctx, events.ObservationCorrected{
			LocationID: location.ID, ObservedAt: c.observedAt,
			SupersededObservationID: c.oldID, NewObservationID: c.newID,
		})
	}
	log.InfoContext(ctx, "observation.collected",
		slog.String("outcome", string(result.Outcome)),
		slog.Int("records_received", result.RecordsReceived),
		slog.Int("stored", stored),
		slog.Int("suspect", suspectStored),
		slog.Int("corrections", len(corrections)))
}

// observationMonthStarts returns the distinct first-of-month instants (UTC)
// covering the observations' observed_at — the partitions that must exist.
func observationMonthStarts(obs []*domain.Observation) []time.Time {
	seen := map[time.Time]struct{}{}
	var out []time.Time
	for _, o := range obs {
		t := o.ObservedAt.UTC()
		ms := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
		if _, ok := seen[ms]; !ok {
			seen[ms] = struct{}{}
			out = append(out, ms)
		}
	}
	return out
}

// newestObservedAt returns the latest observed_at across the fetched rows.
func newestObservedAt(obs []*domain.Observation) time.Time {
	var latest time.Time
	for _, o := range obs {
		if o.ObservedAt.After(latest) {
			latest = o.ObservedAt.UTC()
		}
	}
	return latest
}
