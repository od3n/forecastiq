package analysispg

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/analysis/domain"
	"github.com/forecastiq/forecastiq/internal/analysis/ports"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// MetricRepository implements ports.MetricRepository on PostgreSQL (WP-13).
type MetricRepository struct{}

// NewMetricRepository returns a MetricRepository.
func NewMetricRepository() *MetricRepository { return &MetricRepository{} }

// ListCells implements ports.MetricRepository.
func (r *MetricRepository) ListCells(ctx context.Context, tx dbtx.DBTX, from, to time.Time) ([]domain.Cell, error) {
	rows, err := tx.Query(ctx,
		`SELECT DISTINCT me.provider_id, me.location_id, me.forecast_horizon_minutes
		 FROM matched_evaluations me
		 JOIN observations o ON o.id = me.observation_id
		 WHERE me.target_time >= $1 AND me.target_time < $2
		   AND o.superseded_observation_id IS NULL`,
		from.UTC(), to.UTC())
	if err != nil {
		return nil, fmt.Errorf("list cells: %w", err)
	}
	defer rows.Close()
	var out []domain.Cell
	for rows.Next() {
		var c domain.Cell
		if err := rows.Scan(&c.ProviderID, &c.LocationID, &c.HorizonMinutes); err != nil {
			return nil, fmt.Errorf("scan cell: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ReadPairs implements ports.MetricRepository. Only pairs to live
// (non-superseded, non-suspect) observations are returned.
func (r *MetricRepository) ReadPairs(ctx context.Context, tx dbtx.DBTX, cell domain.Cell, from, to time.Time) ([]*domain.PairRecord, error) {
	rows, err := tx.Query(ctx,
		`SELECT o.observation_type, o.quality_flag,
		        s.temperature_c, s.wind_speed_ms, s.humidity_pct, s.pressure_hpa,
		        s.precipitation_amount_mm, s.precipitation_probability,
		        o.temperature_c, o.wind_speed_ms, o.humidity_pct, o.pressure_hpa, o.precipitation_mm
		 FROM matched_evaluations me
		 JOIN forecast_snapshots s ON s.id = me.forecast_snapshot_id
		 JOIN observations o ON o.id = me.observation_id
		 WHERE me.provider_id = $1 AND me.location_id = $2 AND me.forecast_horizon_minutes = $3
		   AND me.target_time >= $4 AND me.target_time < $5
		   AND o.superseded_observation_id IS NULL AND o.quality_flag <> 'suspect'`,
		cell.ProviderID, cell.LocationID, cell.HorizonMinutes, from.UTC(), to.UTC())
	if err != nil {
		return nil, fmt.Errorf("read pairs: %w", err)
	}
	defer rows.Close()
	var out []*domain.PairRecord
	for rows.Next() {
		var p domain.PairRecord
		if err := rows.Scan(&p.ObservationType, &p.QualityFlag,
			&p.FTemperature, &p.FWindSpeed, &p.FHumidity, &p.FPressure, &p.FPrecipMM, &p.FPrecipProb,
			&p.OTemperature, &p.OWindSpeed, &p.OHumidity, &p.OPressure, &p.OPrecipMM); err != nil {
			return nil, fmt.Errorf("scan pair: %w", err)
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

// SnapshotVariableCounts implements ports.MetricRepository.
func (r *MetricRepository) SnapshotVariableCounts(ctx context.Context, tx dbtx.DBTX, cell domain.Cell, from, to time.Time) (ports.VariableCounts, error) {
	var v ports.VariableCounts
	err := tx.QueryRow(ctx,
		`SELECT count(temperature_c), count(wind_speed_ms), count(humidity_pct),
		        count(pressure_hpa), count(precipitation_amount_mm)
		 FROM forecast_snapshots
		 WHERE provider_id = $1 AND location_id = $2 AND forecast_horizon_minutes = $3
		   AND target_time >= $4 AND target_time < $5`,
		cell.ProviderID, cell.LocationID, cell.HorizonMinutes, from.UTC(), to.UTC()).
		Scan(&v.Temperature, &v.WindSpeed, &v.Humidity, &v.Pressure, &v.Precipitation)
	if err != nil {
		return ports.VariableCounts{}, fmt.Errorf("snapshot variable counts: %w", err)
	}
	return v, nil
}

// ScheduledForecastSlots implements ports.MetricRepository.
func (r *MetricRepository) ScheduledForecastSlots(ctx context.Context, tx dbtx.DBTX, providerID, locationID uuid.UUID, from, to time.Time) (int, error) {
	var n int
	err := tx.QueryRow(ctx,
		`SELECT count(*)
		 FROM collection_schedules cs
		 JOIN provider_configurations pc ON pc.id = cs.provider_configuration_id
		 WHERE cs.job_type = 'forecast_collection' AND cs.location_id = $2
		   AND pc.provider_id = $1 AND cs.slot_time >= $3 AND cs.slot_time < $4`,
		providerID, locationID, from.UTC(), to.UTC()).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("scheduled forecast slots: %w", err)
	}
	return n, nil
}

// SuccessfulCollections implements ports.MetricRepository.
func (r *MetricRepository) SuccessfulCollections(ctx context.Context, tx dbtx.DBTX, providerID, locationID uuid.UUID, from, to time.Time) (int, error) {
	var n int
	err := tx.QueryRow(ctx,
		`SELECT count(*) FROM forecast_collections
		 WHERE provider_id = $1 AND location_id = $2 AND collection_status = 'success'
		   AND requested_at >= $3 AND requested_at < $4`,
		providerID, locationID, from.UTC(), to.UTC()).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("successful collections: %w", err)
	}
	return n, nil
}

// InsertMetrics implements ports.MetricRepository (multi-row).
func (r *MetricRepository) InsertMetrics(ctx context.Context, tx dbtx.DBTX, metrics []*domain.AccuracyMetric) error {
	if len(metrics) == 0 {
		return nil
	}
	const perRow = 13
	var sb strings.Builder
	sb.WriteString(`INSERT INTO accuracy_metrics
		(id, provider_id, location_id, horizon_minutes, variable, metric_type,
		 value, ci_lower, ci_upper, sample_count, methodology_version, period_start, period_end) VALUES `)
	args := make([]any, 0, len(metrics)*perRow)
	for i, m := range metrics {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("(")
		for j := 0; j < perRow; j++ {
			if j > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(&sb, "$%d", i*perRow+j+1)
		}
		sb.WriteString(")")
		args = append(args, m.ID, m.ProviderID, m.LocationID, m.HorizonMinutes, m.Variable, m.MetricType,
			m.Value, m.CILower, m.CIUpper, m.SampleCount, m.MethodologyVersion, m.PeriodStart.UTC(), m.PeriodEnd.UTC())
	}
	if _, err := tx.Exec(ctx, sb.String(), args...); err != nil {
		return fmt.Errorf("insert metrics: %w", err)
	}
	return nil
}

// SupersedePrevious implements ports.MetricRepository.
func (r *MetricRepository) SupersedePrevious(ctx context.Context, tx dbtx.DBTX, m *domain.AccuracyMetric) error {
	_, err := tx.Exec(ctx,
		`UPDATE accuracy_metrics SET superseded_by = $1
		 WHERE provider_id = $2 AND location_id = $3 AND horizon_minutes = $4
		   AND variable = $5 AND metric_type = $6 AND period_start = $7 AND period_end = $8
		   AND superseded_by IS NULL AND id <> $1`,
		m.ID, m.ProviderID, m.LocationID, m.HorizonMinutes, m.Variable, m.MetricType,
		m.PeriodStart.UTC(), m.PeriodEnd.UTC())
	if err != nil {
		return fmt.Errorf("supersede previous metric: %w", err)
	}
	return nil
}
