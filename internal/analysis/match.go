// Package analysis is the analysis module's application layer. WP-11 provides
// the matching engine: a deterministic, chunked batch that pairs unmatched
// forecast snapshots with their best observation for the target hour, plus a
// rematch pass that adds new pairs when a matched observation was superseded by
// a correction (ADR-014; docs/workflows/03-matching.md).
package analysis

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/analysis/domain"
	"github.com/forecastiq/forecastiq/internal/analysis/ports"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
	"github.com/forecastiq/forecastiq/internal/platform/ids"
	"github.com/forecastiq/forecastiq/internal/platform/metrics"
)

const (
	defaultChunkSize  = 5000 // snapshots per tx (workflow §7)
	matchWindowBack   = 30 * 24 * time.Hour
	publicationMargin = 2 * time.Hour // don't match the freshest hours yet
	rematchBatchLimit = 5000
)

// MatchService implements the matching engine (workflow §2/§5).
type MatchService struct {
	repo      ports.MatchRepository
	tx        *dbtx.Runner
	pool      dbtx.DBTX
	metrics   *metrics.Metrics
	clock     clock.Clock
	logger    *slog.Logger
	chunkSize int
}

// NewMatchService wires a MatchService.
func NewMatchService(repo ports.MatchRepository, tx *dbtx.Runner, pool dbtx.DBTX,
	m *metrics.Metrics, clk clock.Clock, logger *slog.Logger) *MatchService {
	return &MatchService{
		repo: repo, tx: tx, pool: pool, metrics: m, clock: clk, logger: logger,
		chunkSize: defaultChunkSize,
	}
}

// MatchBatch runs one matching batch: match unmatched snapshots in
// [now−30d, now−2h] (chunked, per-chunk tx for failure isolation), then rematch
// pairs whose observation was superseded. Returns the number of pairs created.
// Idempotent: re-runs create zero new pairs (uniqueness + NOT EXISTS scan).
func (s *MatchService) MatchBatch(ctx context.Context) (int, error) {
	now := s.clock.Now().UTC()
	from := now.Add(-matchWindowBack)
	to := now.Add(-publicationMargin)
	start := s.clock.Now()

	created := 0
	after := uuid.Nil
	scanned := 0
	for {
		chunk, err := s.repo.ListUnmatchedSnapshots(ctx, s.pool, from, to, after, s.chunkSize)
		if err != nil {
			return created, err
		}
		if len(chunk) == 0 {
			break
		}
		scanned += len(chunk)
		matches := make([]*domain.MatchedEvaluation, 0, len(chunk))
		for _, snap := range chunk {
			hour := snap.TargetTime.UTC().Truncate(time.Hour)
			candidates, err := s.repo.FindCandidates(ctx, s.pool, snap.LocationID, hour)
			if err != nil {
				return created, err
			}
			chosen := domain.SelectCandidate(hour, candidates)
			if chosen == nil {
				continue // no observation yet — remains unmatched (coverage)
			}
			matches = append(matches, newPair(*snap, chosen.ID, chosen.ObservedAt))
		}
		if len(matches) > 0 {
			var n int
			if err := s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
				var e error
				n, e = s.repo.InsertMatches(ctx, tx, matches)
				return e
			}); err != nil {
				return created, err
			}
			created += n
		}
		after = chunk[len(chunk)-1].ID
		if len(chunk) < s.chunkSize {
			break
		}
	}

	rematched, err := s.rematch(ctx)
	if err != nil {
		return created, err
	}
	created += rematched

	s.updateBacklog(ctx, from, to)
	if s.metrics != nil && created > 0 {
		s.metrics.MatchesCreated.Add(float64(created))
	}
	s.logger.InfoContext(ctx, "matching.batch_completed",
		slog.Int("pairs_created", created), slog.Int("snapshots_scanned", scanned), slog.Int("rematched", rematched),
		slog.Duration("duration", s.clock.Now().Sub(start)))
	return created, nil
}

// rematch adds new pairs for matches whose observation was superseded by a
// correction (workflow §5). Old pairs are retained (lineage); the new pair
// points to the correcting observation. Bounded per batch (idempotent).
func (s *MatchService) rematch(ctx context.Context) (int, error) {
	targets, err := s.repo.ListRematchTargets(ctx, s.pool, rematchBatchLimit)
	if err != nil {
		return 0, err
	}
	if len(targets) == 0 {
		return 0, nil
	}
	matches := make([]*domain.MatchedEvaluation, 0, len(targets))
	for _, t := range targets {
		matches = append(matches, newPair(t.Snapshot, t.NewObservationID, t.NewObservedAt))
	}
	var n int
	if err := s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		var e error
		n, e = s.repo.InsertMatches(ctx, tx, matches)
		return e
	}); err != nil {
		return 0, err
	}
	return n, nil
}

// updateBacklog refreshes the matching_backlog gauge (best-effort; a failure
// here must not fail the batch).
func (s *MatchService) updateBacklog(ctx context.Context, from, to time.Time) {
	if s.metrics == nil {
		return
	}
	n, err := s.repo.CountUnmatched(ctx, s.pool, from, to)
	if err != nil {
		s.logger.WarnContext(ctx, "matching.backlog_count_failed", slog.String("error", err.Error()))
		return
	}
	s.metrics.MatchingBacklog.Set(float64(n))
}

// newPair builds an exact-hour MatchedEvaluation for a snapshot and chosen
// observation. time_delta_minutes is |observed_at − target_time| (0 for
// exact-hour hourly sources).
func newPair(snap domain.SnapshotToMatch, obsID uuid.UUID, observedAt time.Time) *domain.MatchedEvaluation {
	delta := int(observedAt.UTC().Sub(snap.TargetTime.UTC()).Minutes())
	if delta < 0 {
		delta = -delta
	}
	return &domain.MatchedEvaluation{
		ID:                     ids.New(),
		ForecastSnapshotID:     snap.ID,
		ObservationID:          obsID,
		ProviderID:             snap.ProviderID,
		LocationID:             snap.LocationID,
		ForecastHorizonMinutes: snap.ForecastHorizonMinutes,
		TargetTime:             snap.TargetTime,
		MatchRule:              domain.MatchExactHour,
		TimeDeltaMinutes:       delta,
	}
}
