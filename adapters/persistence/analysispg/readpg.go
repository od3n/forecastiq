package analysispg

import (
	"context"
	"errors"
	"fmt"
	"time"

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

// aggregationSpanSQL maps an aggregation label to the period-span predicate that
// isolates the matching stored rows (accuracy_metrics has no period-kind column;
// rows are distinguished by their period_end − period_start span). The label is
// validated by the service against a closed set, so this is never user input.
func aggregationSpanSQL(aggregation string) string {
	switch aggregation {
	case "weekly":
		return "period_end = period_start + interval '7 days'"
	case "monthly":
		return "period_end = period_start + interval '1 month'"
	default: // daily
		return "period_end = period_start + interval '1 day'"
	}
}

// LocationMetrics implements ports.ReadRepository: live monthly-period metric
// rows for a location + horizon at the latest stored period.
func (r *ReadRepository) LocationMetrics(ctx context.Context, tx dbtx.DBTX, locationID uuid.UUID, horizonMinutes int) ([]*ports.MetricRow, error) {
	rows, err := tx.Query(ctx,
		`SELECT provider_id, variable, metric_type, value, ci_lower, ci_upper, sample_count
		 FROM accuracy_metrics
		 WHERE location_id = $1 AND horizon_minutes = $2 AND superseded_by IS NULL
		   AND period_end = period_start + interval '1 month'
		   AND period_start = (
		     SELECT max(period_start) FROM accuracy_metrics
		     WHERE location_id = $1 AND horizon_minutes = $2 AND superseded_by IS NULL
		       AND period_end = period_start + interval '1 month')
		 ORDER BY provider_id, variable, metric_type`,
		locationID, horizonMinutes)
	if err != nil {
		return nil, fmt.Errorf("location metrics: %w", err)
	}
	defer rows.Close()
	var out []*ports.MetricRow
	for rows.Next() {
		var m ports.MetricRow
		if err := rows.Scan(&m.ProviderID, &m.Variable, &m.MetricType, &m.Value, &m.CILower, &m.CIUpper, &m.SampleCount); err != nil {
			return nil, fmt.Errorf("scan location metric: %w", err)
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}

// LocationProviderStatuses implements ports.ReadRepository.
func (r *ReadRepository) LocationProviderStatuses(ctx context.Context, tx dbtx.DBTX, locationID uuid.UUID, horizonMinutes int) (map[uuid.UUID]ports.ProviderStatus, error) {
	rows, err := tx.Query(ctx,
		`SELECT provider_id, ranking_status, coverage, reliability
		 FROM provider_rankings
		 WHERE location_id = $1 AND horizon_minutes = $2 AND horizon_profile = 'uniform'
		   AND superseded_by IS NULL
		   AND period_start = (
		     SELECT max(period_start) FROM provider_rankings
		     WHERE location_id = $1 AND horizon_minutes = $2 AND horizon_profile = 'uniform'
		       AND superseded_by IS NULL)`,
		locationID, horizonMinutes)
	if err != nil {
		return nil, fmt.Errorf("location provider statuses: %w", err)
	}
	defer rows.Close()
	out := map[uuid.UUID]ports.ProviderStatus{}
	for rows.Next() {
		var id uuid.UUID
		var st ports.ProviderStatus
		if err := rows.Scan(&id, &st.RankingStatus, &st.Coverage, &st.Reliability); err != nil {
			return nil, fmt.Errorf("scan provider status: %w", err)
		}
		out[id] = st
	}
	return out, rows.Err()
}

// LocationWindows implements ports.ReadRepository: MIN/MAX snapshot target_time
// per provider at a location.
func (r *ReadRepository) LocationWindows(ctx context.Context, tx dbtx.DBTX, locationID uuid.UUID) (map[uuid.UUID]ports.CollectionWindow, error) {
	rows, err := tx.Query(ctx,
		`SELECT provider_id, min(target_time), max(target_time)
		 FROM forecast_snapshots WHERE location_id = $1 GROUP BY provider_id`,
		locationID)
	if err != nil {
		return nil, fmt.Errorf("location windows: %w", err)
	}
	defer rows.Close()
	out := map[uuid.UUID]ports.CollectionWindow{}
	for rows.Next() {
		var id uuid.UUID
		var w ports.CollectionWindow
		if err := rows.Scan(&id, &w.FirstSnapshotAt, &w.LastSnapshotAt); err != nil {
			return nil, fmt.Errorf("scan location window: %w", err)
		}
		out[id] = w
	}
	return out, rows.Err()
}

// ProviderRankingCells implements ports.ReadRepository.
func (r *ReadRepository) ProviderRankingCells(ctx context.Context, tx dbtx.DBTX, providerID uuid.UUID) ([]*ports.ProviderSummaryCell, error) {
	rows, err := tx.Query(ctx,
		`SELECT pr.location_id, l.name, pr.horizon_minutes, pr.composite_score,
		        pr.ranking_status, pr.sample_count, pr.coverage, pr.reliability
		 FROM provider_rankings pr
		 JOIN locations l ON l.id = pr.location_id
		 WHERE pr.provider_id = $1 AND pr.horizon_profile = 'uniform' AND pr.superseded_by IS NULL
		   AND pr.period_start = (
		     SELECT max(period_start) FROM provider_rankings pr2
		     WHERE pr2.provider_id = pr.provider_id AND pr2.location_id = pr.location_id
		       AND pr2.horizon_minutes = pr.horizon_minutes AND pr2.horizon_profile = 'uniform'
		       AND pr2.superseded_by IS NULL)
		 ORDER BY l.name, pr.horizon_minutes`,
		providerID)
	if err != nil {
		return nil, fmt.Errorf("provider ranking cells: %w", err)
	}
	defer rows.Close()
	var out []*ports.ProviderSummaryCell
	for rows.Next() {
		var c ports.ProviderSummaryCell
		if err := rows.Scan(&c.LocationID, &c.LocationName, &c.HorizonMinutes, &c.CompositeScore,
			&c.RankingStatus, &c.SampleCount, &c.Coverage, &c.Reliability); err != nil {
			return nil, fmt.Errorf("scan provider cell: %w", err)
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}

// ProviderWindows implements ports.ReadRepository: MIN/MAX snapshot target_time
// per location for a provider.
func (r *ReadRepository) ProviderWindows(ctx context.Context, tx dbtx.DBTX, providerID uuid.UUID) (map[uuid.UUID]ports.CollectionWindow, error) {
	rows, err := tx.Query(ctx,
		`SELECT location_id, min(target_time), max(target_time)
		 FROM forecast_snapshots WHERE provider_id = $1 GROUP BY location_id`,
		providerID)
	if err != nil {
		return nil, fmt.Errorf("provider windows: %w", err)
	}
	defer rows.Close()
	out := map[uuid.UUID]ports.CollectionWindow{}
	for rows.Next() {
		var id uuid.UUID
		var w ports.CollectionWindow
		if err := rows.Scan(&id, &w.FirstSnapshotAt, &w.LastSnapshotAt); err != nil {
			return nil, fmt.Errorf("scan provider window: %w", err)
		}
		out[id] = w
	}
	return out, rows.Err()
}

// AccuracyTrends implements ports.ReadRepository. The aggregation span predicate
// is a validated constant; all values are parameterized.
func (r *ReadRepository) AccuracyTrends(ctx context.Context, tx dbtx.DBTX, f ports.TrendFilter) ([]*ports.TrendBucket, error) {
	q := `SELECT provider_id, period_start, period_end, value, ci_lower, ci_upper, sample_count
		 FROM accuracy_metrics
		 WHERE location_id = $1 AND horizon_minutes = $2 AND variable = $3 AND metric_type = $4
		   AND superseded_by IS NULL
		   AND period_start >= $5 AND period_end <= $6
		   AND ` + aggregationSpanSQL(f.Aggregation)
	args := []any{f.LocationID, f.HorizonMinutes, f.Variable, f.MetricType, f.From.UTC(), f.To.UTC()}
	if f.ProviderID != nil {
		q += ` AND provider_id = $7`
		args = append(args, *f.ProviderID)
	}
	q += ` ORDER BY provider_id, period_start`
	if f.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", f.Limit)
	}
	rows, err := tx.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("accuracy trends: %w", err)
	}
	defer rows.Close()
	var out []*ports.TrendBucket
	for rows.Next() {
		var b ports.TrendBucket
		if err := rows.Scan(&b.ProviderID, &b.PeriodStart, &b.PeriodEnd, &b.Value, &b.CILower, &b.CIUpper, &b.SampleCount); err != nil {
			return nil, fmt.Errorf("scan trend bucket: %w", err)
		}
		out = append(out, &b)
	}
	return out, rows.Err()
}

// forecastColumn maps a validated variable to its forecast_snapshots column.
// The service validates the variable against the closed set first, so the
// returned identifier is never raw user input (no injection surface).
func forecastColumn(variable string) (string, bool) {
	switch variable {
	case "temperature":
		return "temperature_c", true
	case "wind_speed":
		return "wind_speed_ms", true
	case "humidity":
		return "humidity_pct", true
	case "pressure":
		return "pressure_hpa", true
	case "precipitation":
		return "precipitation_amount_mm", true
	default:
		return "", false
	}
}

// observationColumn maps a validated variable to its observations column.
func observationColumn(variable string) (string, bool) {
	switch variable {
	case "temperature":
		return "temperature_c", true
	case "wind_speed":
		return "wind_speed_ms", true
	case "humidity":
		return "humidity_pct", true
	case "pressure":
		return "pressure_hpa", true
	case "precipitation":
		return "precipitation_mm", true
	default:
		return "", false
	}
}

// ForecastComparisonPoints implements ports.ReadRepository. DISTINCT ON picks,
// per (provider, target hour), the row with the largest horizon ≤ requested
// (DR-02: exact horizon when present, else the nearest shorter). The variable
// column is a validated constant; all values are parameterized.
func (r *ReadRepository) ForecastComparisonPoints(ctx context.Context, tx dbtx.DBTX, locationID uuid.UUID, providerIDs []uuid.UUID, variable string, horizonMinutes int, from, to time.Time) ([]*ports.ForecastPoint, error) {
	col, ok := forecastColumn(variable)
	if !ok {
		return nil, fmt.Errorf("forecast comparison: unsupported variable %q", variable)
	}
	q := `SELECT DISTINCT ON (provider_id, target_time)
	        provider_id, target_time, issued_at, forecast_horizon_minutes, ` + col + `
	      FROM forecast_snapshots
	      WHERE location_id = $1 AND provider_id = ANY($2)
	        AND target_time >= $3 AND target_time < $4
	        AND forecast_horizon_minutes <= $5 AND ` + col + ` IS NOT NULL
	      ORDER BY provider_id, target_time, forecast_horizon_minutes DESC`
	rows, err := tx.Query(ctx, q, locationID, providerIDs, from.UTC(), to.UTC(), horizonMinutes)
	if err != nil {
		return nil, fmt.Errorf("forecast comparison points: %w", err)
	}
	defer rows.Close()
	var out []*ports.ForecastPoint
	for rows.Next() {
		var p ports.ForecastPoint
		if err := rows.Scan(&p.ProviderID, &p.TargetTime, &p.IssuedAt, &p.HorizonMinutes, &p.Value); err != nil {
			return nil, fmt.Errorf("scan forecast point: %w", err)
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

// ComparisonObservations implements ports.ReadRepository.
func (r *ReadRepository) ComparisonObservations(ctx context.Context, tx dbtx.DBTX, locationID uuid.UUID, variable string, from, to time.Time) ([]*ports.ComparisonObservation, error) {
	col, ok := observationColumn(variable)
	if !ok {
		return nil, fmt.Errorf("comparison observations: unsupported variable %q", variable)
	}
	q := `SELECT observed_at, ` + col + `, source, observation_type, quality_flag, canonical_condition_code
	      FROM observations
	      WHERE location_id = $1 AND observed_at >= $2 AND observed_at < $3
	        AND superseded_observation_id IS NULL AND quality_flag <> 'suspect'
	        AND ` + col + ` IS NOT NULL
	      ORDER BY observed_at`
	rows, err := tx.Query(ctx, q, locationID, from.UTC(), to.UTC())
	if err != nil {
		return nil, fmt.Errorf("comparison observations: %w", err)
	}
	defer rows.Close()
	var out []*ports.ComparisonObservation
	for rows.Next() {
		var o ports.ComparisonObservation
		if err := rows.Scan(&o.ObservedAt, &o.Value, &o.Source, &o.ObservationType, &o.QualityFlag, &o.ConditionCode); err != nil {
			return nil, fmt.Errorf("scan comparison observation: %w", err)
		}
		out = append(out, &o)
	}
	return out, rows.Err()
}
