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
}
