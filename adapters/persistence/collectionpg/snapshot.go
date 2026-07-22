package collectionpg

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/forecastiq/forecastiq/internal/collection/domain"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// SnapshotRepository implements ports.SnapshotRepository.
type SnapshotRepository struct{}

// NewSnapshotRepository returns a SnapshotRepository.
func NewSnapshotRepository() *SnapshotRepository { return &SnapshotRepository{} }

const snapshotSelect = `id, forecast_collection_id, provider_id, location_id,
	issued_at, target_time, forecast_horizon_minutes,
	temperature_c, feels_like_temperature_c, precipitation_probability, precipitation_amount_mm,
	humidity_pct, wind_speed_ms, wind_direction_deg, pressure_hpa, cloud_cover_pct,
	COALESCE(provider_condition_code,''), COALESCE(canonical_condition_code,''),
	COALESCE(condition_taxonomy_version,''), created_at`

func scanSnapshot(row pgx.Row) (*domain.ForecastSnapshot, error) {
	var s domain.ForecastSnapshot
	err := row.Scan(&s.ID, &s.ForecastCollectionID, &s.ProviderID, &s.LocationID,
		&s.IssuedAt, &s.TargetTime, &s.ForecastHorizonMinutes,
		&s.TemperatureC, &s.FeelsLikeTemperatureC, &s.PrecipitationProbability, &s.PrecipitationAmountMM,
		&s.HumidityPct, &s.WindSpeedMS, &s.WindDirectionDeg, &s.PressureHPa, &s.CloudCoverPct,
		&s.ProviderConditionCode, &s.CanonicalConditionCode, &s.ConditionTaxonomyVersion, &s.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("scan snapshot: %w", err)
	}
	return &s, nil
}

// EnsurePartitions implements ports.SnapshotRepository: idempotently creates
// the monthly partitions covering the given month-start instants.
func (r *SnapshotRepository) EnsurePartitions(ctx context.Context, tx dbtx.DBTX, monthStarts []time.Time) error {
	for _, ms := range monthStarts {
		if _, err := tx.Exec(ctx, `SELECT create_monthly_partition('forecast_snapshots', $1::date)`, ms); err != nil {
			return fmt.Errorf("ensure partition %s: %w", ms.Format("2006-01"), err)
		}
	}
	return nil
}

// InsertBatch implements ports.SnapshotRepository: a single multi-row INSERT
// with ON CONFLICT DO NOTHING against the snapshot dedup boundary; returns the
// number of rows actually stored (domain §4.3 idempotent storage).
func (r *SnapshotRepository) InsertBatch(ctx context.Context, tx dbtx.DBTX, snapshots []*domain.ForecastSnapshot) (int, error) {
	if len(snapshots) == 0 {
		return 0, nil
	}
	const cols = `(id, forecast_collection_id, provider_id, location_id, issued_at, target_time,
		forecast_horizon_minutes, temperature_c, feels_like_temperature_c, precipitation_probability,
		precipitation_amount_mm, humidity_pct, wind_speed_ms, wind_direction_deg, pressure_hpa,
		cloud_cover_pct, provider_condition_code, canonical_condition_code, condition_taxonomy_version)`
	const perRow = 19

	var sb strings.Builder
	sb.WriteString(`INSERT INTO forecast_snapshots ` + cols + ` VALUES `)
	args := make([]any, 0, len(snapshots)*perRow)
	for i, s := range snapshots {
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
		args = append(args,
			s.ID, s.ForecastCollectionID, s.ProviderID, s.LocationID, s.IssuedAt, s.TargetTime,
			s.ForecastHorizonMinutes, s.TemperatureC, s.FeelsLikeTemperatureC, s.PrecipitationProbability,
			s.PrecipitationAmountMM, s.HumidityPct, s.WindSpeedMS, s.WindDirectionDeg, s.PressureHPa,
			s.CloudCoverPct, s.ProviderConditionCode, s.CanonicalConditionCode, s.ConditionTaxonomyVersion)
	}
	sb.WriteString(` ON CONFLICT (provider_id, location_id, issued_at, target_time) DO NOTHING`)

	tag, err := tx.Exec(ctx, sb.String(), args...)
	if err != nil {
		return 0, fmt.Errorf("insert snapshots: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// ByCollectionID implements ports.SnapshotRepository.
func (r *SnapshotRepository) ByCollectionID(ctx context.Context, tx dbtx.DBTX, collectionID uuid.UUID) ([]*domain.ForecastSnapshot, error) {
	rows, err := tx.Query(ctx, `SELECT `+snapshotSelect+` FROM forecast_snapshots WHERE forecast_collection_id = $1 ORDER BY target_time`, collectionID)
	if err != nil {
		return nil, fmt.Errorf("list snapshots: %w", err)
	}
	defer rows.Close()
	var out []*domain.ForecastSnapshot
	for rows.Next() {
		s, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetByID implements ports.SnapshotRepository.
func (r *SnapshotRepository) GetByID(ctx context.Context, tx dbtx.DBTX, id uuid.UUID) (*domain.ForecastSnapshot, error) {
	return scanSnapshot(tx.QueryRow(ctx, `SELECT `+snapshotSelect+` FROM forecast_snapshots WHERE id = $1 LIMIT 1`, id))
}
