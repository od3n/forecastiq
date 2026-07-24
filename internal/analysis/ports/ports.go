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

// VariableCounts is the number of delivered snapshots with a non-null value for
// each continuous variable in a cell-period (coverage numerator).
type VariableCounts struct {
	Temperature   int
	WindSpeed     int
	Humidity      int
	Pressure      int
	Precipitation int
}

// MetricRepository is the aggregation engine's persistence port (WP-13).
type MetricRepository interface {
	// ListCells returns the distinct (provider, location, horizon) cells that
	// have at least one live matched pair with target_time in [from, to).
	ListCells(ctx context.Context, tx dbtx.DBTX, from, to time.Time) ([]domain.Cell, error)
	// ReadPairs returns the live matched pairs for a cell whose target_time is in
	// [from, to): matched_evaluations joined to the forecast snapshot and the
	// (non-superseded, non-suspect) observation.
	ReadPairs(ctx context.Context, tx dbtx.DBTX, cell domain.Cell, from, to time.Time) ([]*domain.PairRecord, error)
	// SnapshotVariableCounts returns, per variable, the number of delivered
	// snapshots for the cell with a non-null value in [from, to) (coverage
	// numerator).
	SnapshotVariableCounts(ctx context.Context, tx dbtx.DBTX, cell domain.Cell, from, to time.Time) (VariableCounts, error)
	// ScheduledForecastSlots returns the number of scheduled forecast-collection
	// slots for the provider serving the location in [from, to) (coverage /
	// reliability denominator; schedule-derived expected count).
	ScheduledForecastSlots(ctx context.Context, tx dbtx.DBTX, providerID, locationID uuid.UUID, from, to time.Time) (int, error)
	// SuccessfulCollections returns the number of successful forecast collections
	// for the provider+location in [from, to) (reliability numerator).
	SuccessfulCollections(ctx context.Context, tx dbtx.DBTX, providerID, locationID uuid.UUID, from, to time.Time) (int, error)
	// InsertMetrics inserts new metric rows (live: superseded_by NULL).
	InsertMetrics(ctx context.Context, tx dbtx.DBTX, metrics []*domain.AccuracyMetric) error
	// SupersedePrevious sets superseded_by = m.ID on the prior live row sharing
	// m's logical key (provider, location, horizon, variable, metric_type,
	// period), if any. Called after the new row is inserted.
	SupersedePrevious(ctx context.Context, tx dbtx.DBTX, m *domain.AccuracyMetric) error
}

// LocationHorizon identifies a ranking cell's location and horizon.
type LocationHorizon struct {
	LocationID     uuid.UUID
	HorizonMinutes int
}

// MetricValue is one live accuracy_metrics row read for ranking.
type MetricValue struct {
	ProviderID  uuid.UUID
	Variable    string
	MetricType  string
	Value       *float64
	CILower     *float64
	CIUpper     *float64
	SampleCount int
}

// RankingRow is a provider_rankings row to persist (WP-14).
type RankingRow struct {
	ID                  uuid.UUID
	ProviderID          uuid.UUID
	LocationID          uuid.UUID
	HorizonMinutes      int
	CompositeScore      *float64
	CILower             *float64
	CIUpper             *float64
	Status              string
	SampleCount         int
	Coverage            *float64
	Reliability         *float64
	ComponentScoresJSON []byte
	MethodologyVersion  string
	WeightsVersion      string
	HorizonProfile      string
	PeriodStart         time.Time
	PeriodEnd           time.Time
}

// RankingRepository is the ranking engine's persistence port (WP-14).
type RankingRepository interface {
	// ListRankingCells returns the (location, horizon) cells with live accuracy
	// metrics for the exact period.
	ListRankingCells(ctx context.Context, tx dbtx.DBTX, from, to time.Time) ([]LocationHorizon, error)
	// ReadCohortMetrics returns every live accuracy-metric value for a cell-period
	// across all providers (the ranking cohort inputs).
	ReadCohortMetrics(ctx context.Context, tx dbtx.DBTX, locationID uuid.UUID, horizon int, from, to time.Time) ([]MetricValue, error)
	// InsertRankings inserts new ranking rows (live: superseded_by NULL).
	InsertRankings(ctx context.Context, tx dbtx.DBTX, rows []*RankingRow) error
	// SupersedePreviousRankings sets superseded_by = row.ID on the prior live row
	// sharing row's logical key (provider, location, horizon, profile, period).
	SupersedePreviousRankings(ctx context.Context, tx dbtx.DBTX, row *RankingRow) error
}
