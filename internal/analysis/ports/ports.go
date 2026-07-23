// Package ports declares the analysis module's contracts. The matching engine
// depends only on these; the PostgreSQL implementation lives in
// adapters/persistence/analysispg (binding rule).
package ports

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/analysis/domain"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// RematchTarget is an existing pair whose observation was superseded by a
// correction, plus the correcting observation to pair the snapshot with
// (workflow §5). The engine inserts a NEW pair (old retained for lineage).
type RematchTarget struct {
	Snapshot         domain.SnapshotToMatch
	NewObservationID uuid.UUID
	NewObservedAt    time.Time
}

// MatchRepository is the matching engine's persistence port.
type MatchRepository interface {
	// ListUnmatchedSnapshots returns snapshots with target_time in [from, to)
	// that have no matched_evaluations row, keyset-paginated by id (id > after),
	// ordered by id, limited to limit (chunking; workflow §2/§7).
	ListUnmatchedSnapshots(ctx context.Context, tx dbtx.DBTX, from, to time.Time, after uuid.UUID, limit int) ([]*domain.SnapshotToMatch, error)
	// FindCandidates returns live (non-suspect, non-superseded) observations for
	// the location whose observed_at equals hour (exact-hour, MVP).
	FindCandidates(ctx context.Context, tx dbtx.DBTX, locationID uuid.UUID, hour time.Time) ([]*domain.ObservationCandidate, error)
	// InsertMatches inserts pairs with ON CONFLICT (forecast_snapshot_id,
	// observation_id) DO NOTHING; returns the number actually stored.
	InsertMatches(ctx context.Context, tx dbtx.DBTX, matches []*domain.MatchedEvaluation) (int, error)
	// ListRematchTargets returns pairs whose observation is superseded and that
	// do not yet have a pair to the correcting observation (limit-bounded).
	ListRematchTargets(ctx context.Context, tx dbtx.DBTX, limit int) ([]*RematchTarget, error)
	// CountUnmatched returns the number of unmatched snapshots in [from, to)
	// (matching backlog gauge).
	CountUnmatched(ctx context.Context, tx dbtx.DBTX, from, to time.Time) (int, error)
}
