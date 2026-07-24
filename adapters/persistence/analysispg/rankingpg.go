package analysispg

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/analysis/ports"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// RankingRepository implements ports.RankingRepository on PostgreSQL (WP-14).
type RankingRepository struct{}

// NewRankingRepository returns a RankingRepository.
func NewRankingRepository() *RankingRepository { return &RankingRepository{} }

// ListRankingCells implements ports.RankingRepository.
func (r *RankingRepository) ListRankingCells(ctx context.Context, tx dbtx.DBTX, from, to time.Time) ([]ports.LocationHorizon, error) {
	rows, err := tx.Query(ctx,
		`SELECT DISTINCT location_id, horizon_minutes FROM accuracy_metrics
		 WHERE period_start = $1 AND period_end = $2 AND superseded_by IS NULL`,
		from.UTC(), to.UTC())
	if err != nil {
		return nil, fmt.Errorf("list ranking cells: %w", err)
	}
	defer rows.Close()
	var out []ports.LocationHorizon
	for rows.Next() {
		var lh ports.LocationHorizon
		if err := rows.Scan(&lh.LocationID, &lh.HorizonMinutes); err != nil {
			return nil, fmt.Errorf("scan ranking cell: %w", err)
		}
		out = append(out, lh)
	}
	return out, rows.Err()
}

// ReadCohortMetrics implements ports.RankingRepository.
func (r *RankingRepository) ReadCohortMetrics(ctx context.Context, tx dbtx.DBTX, locationID uuid.UUID, horizon int, from, to time.Time) ([]ports.MetricValue, error) {
	rows, err := tx.Query(ctx,
		`SELECT provider_id, variable, metric_type, value, ci_lower, ci_upper, sample_count
		 FROM accuracy_metrics
		 WHERE location_id = $1 AND horizon_minutes = $2 AND period_start = $3 AND period_end = $4
		   AND superseded_by IS NULL`,
		locationID, horizon, from.UTC(), to.UTC())
	if err != nil {
		return nil, fmt.Errorf("read cohort metrics: %w", err)
	}
	defer rows.Close()
	var out []ports.MetricValue
	for rows.Next() {
		var m ports.MetricValue
		if err := rows.Scan(&m.ProviderID, &m.Variable, &m.MetricType, &m.Value, &m.CILower, &m.CIUpper, &m.SampleCount); err != nil {
			return nil, fmt.Errorf("scan cohort metric: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// InsertRankings implements ports.RankingRepository (multi-row).
func (r *RankingRepository) InsertRankings(ctx context.Context, tx dbtx.DBTX, rankings []*ports.RankingRow) error {
	if len(rankings) == 0 {
		return nil
	}
	const perRow = 17
	var sb strings.Builder
	sb.WriteString(`INSERT INTO provider_rankings
		(id, provider_id, location_id, horizon_minutes, composite_score, ci_lower, ci_upper,
		 ranking_status, sample_count, coverage, reliability, component_scores,
		 methodology_version, weights_version, horizon_profile, period_start, period_end) VALUES `)
	args := make([]any, 0, len(rankings)*perRow)
	for i, r := range rankings {
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
		args = append(args, r.ID, r.ProviderID, r.LocationID, r.HorizonMinutes, r.CompositeScore, r.CILower, r.CIUpper,
			r.Status, r.SampleCount, r.Coverage, r.Reliability, r.ComponentScoresJSON,
			r.MethodologyVersion, r.WeightsVersion, r.HorizonProfile, r.PeriodStart.UTC(), r.PeriodEnd.UTC())
	}
	if _, err := tx.Exec(ctx, sb.String(), args...); err != nil {
		return fmt.Errorf("insert rankings: %w", err)
	}
	return nil
}

// SupersedePreviousRankings implements ports.RankingRepository.
func (r *RankingRepository) SupersedePreviousRankings(ctx context.Context, tx dbtx.DBTX, row *ports.RankingRow) error {
	_, err := tx.Exec(ctx,
		`UPDATE provider_rankings SET superseded_by = $1
		 WHERE provider_id = $2 AND location_id = $3 AND horizon_minutes = $4 AND horizon_profile = $5
		   AND period_start = $6 AND period_end = $7 AND superseded_by IS NULL AND id <> $1`,
		row.ID, row.ProviderID, row.LocationID, row.HorizonMinutes, row.HorizonProfile,
		row.PeriodStart.UTC(), row.PeriodEnd.UTC())
	if err != nil {
		return fmt.Errorf("supersede previous ranking: %w", err)
	}
	return nil
}
