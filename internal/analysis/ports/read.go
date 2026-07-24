package ports

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// RankingReadRow is one live provider_rankings row joined to the provider's
// public identity + attribution (the read surface for GET /rankings and the
// provider-mode /accuracy/summary; WP-15). Rounding is applied at the API layer.
type RankingReadRow struct {
	ProviderID          uuid.UUID
	ProviderName        string
	ProviderSlug        string
	AttributionText     string
	AttributionURL      string
	LocationID          uuid.UUID
	LocationName        string
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
	CalculatedAt        time.Time
}

// ObservationContextRow is the latest live (non-superseded, non-suspect)
// observation for a location — the S-01 ground-truth context line (C-01/DR-09).
// It is an observation record (provenance-labeled evidence), never a weather
// product (NP-01). Nil when the location has no observations.
type ObservationContextRow struct {
	TemperatureC    *float64
	PrecipitationMM *float64
	ObservedAt      time.Time
	Source          string
	ObservationType string
	QualityFlag     string
}

// MetricRow is one live accuracy_metrics value (a cell of the S-02 grid).
type MetricRow struct {
	ProviderID  uuid.UUID
	Variable    string
	MetricType  string
	Value       *float64
	CILower     *float64
	CIUpper     *float64
	SampleCount int
}

// CollectionWindow is the temporal + quality window of a provider's stored data
// for a location (C-08): first/last snapshot target_time and the ranking's
// coverage/reliability. Fields are nil when no data/ranking exists.
type CollectionWindow struct {
	FirstSnapshotAt *time.Time
	LastSnapshotAt  *time.Time
	Coverage        *float64
	Reliability     *float64
}

// ProviderStatus carries a provider's live ranking status + coverage/reliability
// at a location+horizon (drives the S-02 per-provider status + window).
type ProviderStatus struct {
	RankingStatus string
	Coverage      *float64
	Reliability   *float64
}

// ProviderSummaryCell is one (location, horizon) ranking cell for a provider
// (S-03 provider-mode grid).
type ProviderSummaryCell struct {
	LocationID     uuid.UUID
	LocationName   string
	HorizonMinutes int
	CompositeScore *float64
	RankingStatus  string
	SampleCount    int
	Coverage       *float64
	Reliability    *float64
}

// TrendFilter selects accuracy_metrics rows for a trend series (GET /accuracy).
// Aggregation selects the stored period span (daily|weekly|monthly).
type TrendFilter struct {
	ProviderID     *uuid.UUID
	LocationID     uuid.UUID
	HorizonMinutes int
	Variable       string
	MetricType     string
	From           time.Time
	To             time.Time
	Aggregation    string
	Limit          int
}

// TrendBucket is one period's metric value in a trend series. SampleCount rides
// with every bucket (hollow-point support; value nil when sample_count 0).
type TrendBucket struct {
	ProviderID  uuid.UUID
	PeriodStart time.Time
	PeriodEnd   time.Time
	Value       *float64
	CILower     *float64
	CIUpper     *float64
	SampleCount int
}

// ReadRepository serves the public dashboard reads over pre-computed analysis
// rows (WP-15). All queries hit the live (superseded_by IS NULL) surface.
type ReadRepository interface {
	// ListRankings returns the live ranking rows for a location + horizon +
	// profile at the most recent stored evaluation period, joined to provider
	// identity. Empty (not an error) when no rankings exist for the cell.
	ListRankings(ctx context.Context, tx dbtx.DBTX, locationID uuid.UUID, horizonMinutes int, profile string) ([]*RankingReadRow, error)
	// LatestObservation returns the newest live observation for a location, or
	// (nil, nil) when none exists.
	LatestObservation(ctx context.Context, tx dbtx.DBTX, locationID uuid.UUID) (*ObservationContextRow, error)
	// LocationMetrics returns every live monthly-period metric row for a
	// location + horizon at the latest stored period (all providers/variables).
	LocationMetrics(ctx context.Context, tx dbtx.DBTX, locationID uuid.UUID, horizonMinutes int) ([]*MetricRow, error)
	// LocationProviderStatuses returns each provider's live ranking status +
	// coverage/reliability for a location + horizon (uniform profile).
	LocationProviderStatuses(ctx context.Context, tx dbtx.DBTX, locationID uuid.UUID, horizonMinutes int) (map[uuid.UUID]ProviderStatus, error)
	// LocationWindows returns each provider's collection window (snapshot
	// target_time MIN/MAX) at a location.
	LocationWindows(ctx context.Context, tx dbtx.DBTX, locationID uuid.UUID) (map[uuid.UUID]CollectionWindow, error)
	// ProviderRankingCells returns every live ranking cell for a provider
	// (uniform profile, latest period per cell), joined to location identity.
	ProviderRankingCells(ctx context.Context, tx dbtx.DBTX, providerID uuid.UUID) ([]*ProviderSummaryCell, error)
	// ProviderWindows returns the provider's collection window per location.
	ProviderWindows(ctx context.Context, tx dbtx.DBTX, providerID uuid.UUID) (map[uuid.UUID]CollectionWindow, error)
	// AccuracyTrends returns the metric buckets matching a filter, ordered by
	// provider then period_start.
	AccuracyTrends(ctx context.Context, tx dbtx.DBTX, f TrendFilter) ([]*TrendBucket, error)
}
