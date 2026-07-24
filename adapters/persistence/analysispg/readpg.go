package analysispg

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/forecastiq/forecastiq/internal/analysis/ports"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// ReadRepository implements ports.ReadRepository on PostgreSQL — the public
// dashboard read surface over pre-computed analysis rows (WP-15). All queries
// are parameterized and hit only live (superseded_by IS NULL) rows.
type ReadRepository struct{}

// NewReadRepository returns a ReadRepository.
func NewReadRepository() *ReadRepository { return &ReadRepository{} }

// ListRankings implements ports.ReadRepository. It serves the cohort at the
// most recent stored evaluation period for the (location, horizon, profile):
// the ranking batch writes one live period at a time, but scoping to
// MAX(period_start) is defensive against overlapping periods.
func (r *ReadRepository) ListRankings(ctx context.Context, tx dbtx.DBTX, locationID uuid.UUID, horizonMinutes int, profile string) ([]*ports.RankingReadRow, error) {
	rows, err := tx.Query(ctx,
		`SELECT pr.provider_id, p.name, p.slug, p.attribution_text, p.attribution_url,
		        pr.location_id, l.name, pr.horizon_minutes,
		        pr.composite_score, pr.ci_lower, pr.ci_upper, pr.ranking_status, pr.sample_count,
		        pr.coverage, pr.reliability, pr.component_scores,
		        pr.methodology_version, pr.weights_version, pr.horizon_profile,
		        pr.period_start, pr.period_end, pr.calculated_at
		 FROM provider_rankings pr
		 JOIN providers p ON p.id = pr.provider_id
		 JOIN locations l ON l.id = pr.location_id
		 WHERE pr.location_id = $1 AND pr.horizon_minutes = $2 AND pr.horizon_profile = $3
		   AND pr.superseded_by IS NULL
		   AND pr.period_start = (
		     SELECT max(period_start) FROM provider_rankings
		     WHERE location_id = $1 AND horizon_minutes = $2 AND horizon_profile = $3
		       AND superseded_by IS NULL)`,
		locationID, horizonMinutes, profile)
	if err != nil {
		return nil, fmt.Errorf("list rankings: %w", err)
	}
	defer rows.Close()
	var out []*ports.RankingReadRow
	for rows.Next() {
		var v ports.RankingReadRow
		if err := rows.Scan(&v.ProviderID, &v.ProviderName, &v.ProviderSlug, &v.AttributionText, &v.AttributionURL,
			&v.LocationID, &v.LocationName, &v.HorizonMinutes,
			&v.CompositeScore, &v.CILower, &v.CIUpper, &v.Status, &v.SampleCount,
			&v.Coverage, &v.Reliability, &v.ComponentScoresJSON,
			&v.MethodologyVersion, &v.WeightsVersion, &v.HorizonProfile,
			&v.PeriodStart, &v.PeriodEnd, &v.CalculatedAt); err != nil {
			return nil, fmt.Errorf("scan ranking: %w", err)
		}
		out = append(out, &v)
	}
	return out, rows.Err()
}

// LatestObservation implements ports.ReadRepository.
func (r *ReadRepository) LatestObservation(ctx context.Context, tx dbtx.DBTX, locationID uuid.UUID) (*ports.ObservationContextRow, error) {
	var o ports.ObservationContextRow
	err := tx.QueryRow(ctx,
		`SELECT temperature_c, precipitation_mm, observed_at, source, observation_type, quality_flag
		 FROM observations
		 WHERE location_id = $1 AND superseded_observation_id IS NULL AND quality_flag <> 'suspect'
		 ORDER BY observed_at DESC
		 LIMIT 1`,
		locationID).Scan(&o.TemperatureC, &o.PrecipitationMM, &o.ObservedAt, &o.Source, &o.ObservationType, &o.QualityFlag)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("latest observation: %w", err)
	}
	return &o, nil
}
