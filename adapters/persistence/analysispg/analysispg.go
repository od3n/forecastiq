// Package analysispg implements the analysis module's MatchRepository on
// PostgreSQL (WP-11). Queries follow docs/workflows/03-matching.md §2 (unmatched
// scan via NOT EXISTS, exact-hour candidate lookup) and §5 (rematch of pairs
// whose observation was superseded). Parameterized throughout.
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

// MatchRepository implements ports.MatchRepository.
type MatchRepository struct{}

// NewMatchRepository returns a MatchRepository.
func NewMatchRepository() *MatchRepository { return &MatchRepository{} }

// ListUnmatchedSnapshots implements ports.MatchRepository.
func (r *MatchRepository) ListUnmatchedSnapshots(ctx context.Context, tx dbtx.DBTX, from, to time.Time, after uuid.UUID, limit int) ([]*domain.SnapshotToMatch, error) {
	rows, err := tx.Query(ctx,
		`SELECT s.id, s.provider_id, s.location_id, s.target_time, s.forecast_horizon_minutes
		 FROM forecast_snapshots s
		 WHERE s.target_time >= $1 AND s.target_time < $2 AND s.id > $3
		   AND NOT EXISTS (SELECT 1 FROM matched_evaluations me WHERE me.forecast_snapshot_id = s.id)
		 ORDER BY s.id
		 LIMIT $4`,
		from.UTC(), to.UTC(), after, limit)
	if err != nil {
		return nil, fmt.Errorf("list unmatched snapshots: %w", err)
	}
	defer rows.Close()
	var out []*domain.SnapshotToMatch
	for rows.Next() {
		var s domain.SnapshotToMatch
		if err := rows.Scan(&s.ID, &s.ProviderID, &s.LocationID, &s.TargetTime, &s.ForecastHorizonMinutes); err != nil {
			return nil, fmt.Errorf("scan snapshot: %w", err)
		}
		out = append(out, &s)
	}
	return out, rows.Err()
}

// FindCandidates implements ports.MatchRepository.
func (r *MatchRepository) FindCandidates(ctx context.Context, tx dbtx.DBTX, locationID uuid.UUID, hour time.Time) ([]*domain.ObservationCandidate, error) {
	rows, err := tx.Query(ctx,
		`SELECT id, observation_type, quality_flag, observed_at
		 FROM observations
		 WHERE location_id = $1 AND observed_at = $2
		   AND quality_flag <> 'suspect' AND superseded_observation_id IS NULL`,
		locationID, hour.UTC())
	if err != nil {
		return nil, fmt.Errorf("find candidates: %w", err)
	}
	defer rows.Close()
	var out []*domain.ObservationCandidate
	for rows.Next() {
		var c domain.ObservationCandidate
		if err := rows.Scan(&c.ID, &c.ObservationType, &c.QualityFlag, &c.ObservedAt); err != nil {
			return nil, fmt.Errorf("scan candidate: %w", err)
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}

// InsertMatches implements ports.MatchRepository (multi-row, dedup by pair).
func (r *MatchRepository) InsertMatches(ctx context.Context, tx dbtx.DBTX, matches []*domain.MatchedEvaluation) (int, error) {
	if len(matches) == 0 {
		return 0, nil
	}
	const cols = `(id, forecast_snapshot_id, observation_id, provider_id, location_id,
		forecast_horizon_minutes, target_time, match_rule, time_delta_minutes)`
	const perRow = 9

	var sb strings.Builder
	sb.WriteString(`INSERT INTO matched_evaluations ` + cols + ` VALUES `)
	args := make([]any, 0, len(matches)*perRow)
	for i, m := range matches {
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
		args = append(args, m.ID, m.ForecastSnapshotID, m.ObservationID, m.ProviderID, m.LocationID,
			m.ForecastHorizonMinutes, m.TargetTime, m.MatchRule, m.TimeDeltaMinutes)
	}
	sb.WriteString(` ON CONFLICT (forecast_snapshot_id, observation_id) DO NOTHING`)

	tag, err := tx.Exec(ctx, sb.String(), args...)
	if err != nil {
		return 0, fmt.Errorf("insert matches: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// ListRematchTargets implements ports.MatchRepository: existing pairs whose
// observation was superseded, with the correcting observation to re-pair to,
// where that new pair does not already exist (workflow §5).
func (r *MatchRepository) ListRematchTargets(ctx context.Context, tx dbtx.DBTX, limit int) ([]*ports.RematchTarget, error) {
	rows, err := tx.Query(ctx,
		`SELECT me.forecast_snapshot_id, me.provider_id, me.location_id,
		        me.forecast_horizon_minutes, me.target_time,
		        o.superseded_observation_id, n.observed_at
		 FROM matched_evaluations me
		 JOIN observations o ON o.id = me.observation_id
		 JOIN observations n ON n.id = o.superseded_observation_id
		 WHERE o.superseded_observation_id IS NOT NULL
		   AND NOT EXISTS (
		     SELECT 1 FROM matched_evaluations me2
		     WHERE me2.forecast_snapshot_id = me.forecast_snapshot_id
		       AND me2.observation_id = o.superseded_observation_id)
		 ORDER BY me.forecast_snapshot_id
		 LIMIT $1`,
		limit)
	if err != nil {
		return nil, fmt.Errorf("list rematch targets: %w", err)
	}
	defer rows.Close()
	var out []*ports.RematchTarget
	for rows.Next() {
		var t ports.RematchTarget
		if err := rows.Scan(&t.Snapshot.ID, &t.Snapshot.ProviderID, &t.Snapshot.LocationID,
			&t.Snapshot.ForecastHorizonMinutes, &t.Snapshot.TargetTime,
			&t.NewObservationID, &t.NewObservedAt); err != nil {
			return nil, fmt.Errorf("scan rematch target: %w", err)
		}
		out = append(out, &t)
	}
	return out, rows.Err()
}

// CountUnmatched implements ports.MatchRepository.
func (r *MatchRepository) CountUnmatched(ctx context.Context, tx dbtx.DBTX, from, to time.Time) (int, error) {
	var n int
	err := tx.QueryRow(ctx,
		`SELECT count(*) FROM forecast_snapshots s
		 WHERE s.target_time >= $1 AND s.target_time < $2
		   AND NOT EXISTS (SELECT 1 FROM matched_evaluations me WHERE me.forecast_snapshot_id = s.id)`,
		from.UTC(), to.UTC()).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count unmatched: %w", err)
	}
	return n, nil
}
