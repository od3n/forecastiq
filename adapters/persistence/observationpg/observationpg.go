// Package observationpg implements the collection module's ObservationRepository
// port on PostgreSQL (WP-10). Observations are monthly-partitioned by
// observed_at; the live-row dedup boundary is a PARTIAL unique index on
// (source, location_id, observed_at) WHERE superseded_observation_id IS NULL,
// so corrections (a new row sharing the same key) coexist with the superseded
// row they replace. Parameterized queries throughout; adapters import ports,
// never handlers (binding rule).
package observationpg

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/forecastiq/forecastiq/internal/collection/domain"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// Repository implements ports.ObservationRepository.
type Repository struct{}

// NewRepository returns a Repository.
func NewRepository() *Repository { return &Repository{} }

const observationSelect = `id, location_id, source, observation_type, observed_at,
	temperature_c, humidity_pct, wind_speed_ms, wind_direction_deg, pressure_hpa, precipitation_mm,
	COALESCE(provider_condition_code,''), COALESCE(canonical_condition_code,''),
	quality_flag, superseded_observation_id, created_at`

func scanObservation(row pgx.Row) (*domain.Observation, error) {
	var o domain.Observation
	var obsType, flag string
	err := row.Scan(&o.ID, &o.LocationID, &o.Source, &obsType, &o.ObservedAt,
		&o.TemperatureC, &o.HumidityPct, &o.WindSpeedMS, &o.WindDirectionDeg, &o.PressureHPa, &o.PrecipitationMM,
		&o.ProviderConditionCode, &o.CanonicalConditionCode, &flag, &o.SupersededObservationID, &o.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("scan observation: %w", err)
	}
	o.ObservationType = domain.ObservationType(obsType)
	o.QualityFlag = domain.QualityFlag(flag)
	return &o, nil
}

// EnsurePartitions implements ports.ObservationRepository.
func (r *Repository) EnsurePartitions(ctx context.Context, tx dbtx.DBTX, monthStarts []time.Time) error {
	for _, ms := range monthStarts {
		if _, err := tx.Exec(ctx, `SELECT create_monthly_partition('observations', $1::date)`, ms); err != nil {
			return fmt.Errorf("ensure partition %s: %w", ms.Format("2006-01"), err)
		}
	}
	return nil
}

// ListCurrentByWindow implements ports.ObservationRepository: the live
// (non-superseded) rows for a (source, location) in [start, end].
func (r *Repository) ListCurrentByWindow(ctx context.Context, tx dbtx.DBTX, source string, locationID uuid.UUID, start, end time.Time) ([]*domain.Observation, error) {
	rows, err := tx.Query(ctx,
		`SELECT `+observationSelect+` FROM observations
		 WHERE source = $1 AND location_id = $2 AND observed_at BETWEEN $3 AND $4
		   AND superseded_observation_id IS NULL
		 ORDER BY observed_at`,
		source, locationID, start.UTC(), end.UTC())
	if err != nil {
		return nil, fmt.Errorf("list observations: %w", err)
	}
	defer rows.Close()
	var out []*domain.Observation
	for rows.Next() {
		o, err := scanObservation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// InsertBatch implements ports.ObservationRepository: a single multi-row INSERT
// with ON CONFLICT DO NOTHING against the partial live-row dedup index; returns
// the number of rows actually stored.
func (r *Repository) InsertBatch(ctx context.Context, tx dbtx.DBTX, obs []*domain.Observation) (int, error) {
	if len(obs) == 0 {
		return 0, nil
	}
	const cols = `(id, location_id, source, observation_type, observed_at,
		temperature_c, humidity_pct, wind_speed_ms, wind_direction_deg, pressure_hpa,
		precipitation_mm, provider_condition_code, canonical_condition_code, quality_flag)`
	const perRow = 14

	var sb strings.Builder
	sb.WriteString(`INSERT INTO observations ` + cols + ` VALUES `)
	args := make([]any, 0, len(obs)*perRow)
	for i, o := range obs {
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
			o.ID, o.LocationID, o.Source, string(o.ObservationType), o.ObservedAt,
			o.TemperatureC, o.HumidityPct, o.WindSpeedMS, o.WindDirectionDeg, o.PressureHPa,
			o.PrecipitationMM, nullIfEmpty(o.ProviderConditionCode), nullIfEmpty(o.CanonicalConditionCode),
			string(o.QualityFlag))
	}
	// Conflict target must name the partial index predicate (DR-05).
	sb.WriteString(` ON CONFLICT (source, location_id, observed_at)
		WHERE superseded_observation_id IS NULL DO NOTHING`)

	tag, err := tx.Exec(ctx, sb.String(), args...)
	if err != nil {
		return 0, fmt.Errorf("insert observations: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// Supersede implements ports.ObservationRepository (the one permitted mutation).
func (r *Repository) Supersede(ctx context.Context, tx dbtx.DBTX, oldID uuid.UUID, oldObservedAt time.Time, newID uuid.UUID) error {
	tag, err := tx.Exec(ctx,
		`UPDATE observations SET superseded_observation_id = $3
		 WHERE id = $1 AND observed_at = $2 AND superseded_observation_id IS NULL`,
		oldID, oldObservedAt.UTC(), newID)
	if err != nil {
		return fmt.Errorf("supersede observation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("supersede observation %s: row not found or already superseded", oldID)
	}
	return nil
}

// LatestObservedAt implements ports.ObservationRepository.
func (r *Repository) LatestObservedAt(ctx context.Context, tx dbtx.DBTX, source string, locationID uuid.UUID) (time.Time, bool, error) {
	var t *time.Time // MAX over zero rows is SQL NULL → nil
	err := tx.QueryRow(ctx,
		`SELECT max(observed_at) FROM observations WHERE source = $1 AND location_id = $2`,
		source, locationID).Scan(&t)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("latest observed_at: %w", err)
	}
	if t == nil {
		return time.Time{}, false, nil
	}
	return t.UTC(), true, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
