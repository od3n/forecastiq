// Package adminpg implements the admin (operations) module's HealthRepository
// on PostgreSQL — the S-10 collection-health aggregates read purely from
// application tables (operations doc §4). Parameterized queries throughout;
// wired only in the composition root.
package adminpg

import (
	"context"
	"fmt"
	"time"

	"github.com/forecastiq/forecastiq/internal/admin"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// HealthRepository implements admin.HealthRepository.
type HealthRepository struct{}

// NewHealthRepository returns a HealthRepository.
func NewHealthRepository() *HealthRepository { return &HealthRepository{} }

// Cells implements admin.HealthRepository. Each provider×location cell served
// by a forecast schedule carries its last successful collection, most recent
// terminal status, and next future scheduled slot. Slots are generated
// just-in-time, so when none is pending the next run is derived from the
// active configuration's hourly schedule (minute_offset).
func (r *HealthRepository) Cells(ctx context.Context, tx dbtx.DBTX, now time.Time) ([]admin.CellHealth, error) {
	rows, err := tx.Query(ctx,
		`SELECT p.id, p.name, p.slug, l.id, l.name,
		        (SELECT max(fc.completed_at) FROM forecast_collections fc
		           WHERE fc.provider_id = p.id AND fc.location_id = l.id
		             AND fc.collection_status IN ('success','partial')) AS last_success,
		        (SELECT fc.collection_status FROM forecast_collections fc
		           WHERE fc.provider_id = p.id AND fc.location_id = l.id
		           ORDER BY fc.requested_at DESC LIMIT 1) AS last_status,
		        COALESCE(
		          (SELECT min(cs.slot_time) FROM collection_schedules cs
		             JOIN provider_configurations pc2 ON pc2.id = cs.provider_configuration_id
		             WHERE cs.job_type = 'forecast_collection' AND cs.location_id = l.id
		               AND pc2.provider_id = p.id AND cs.slot_time > $1
		               AND cs.status IN ('due','claimed')),
		          (SELECT CASE
		             WHEN date_trunc('hour', $1::timestamptz)
		                  + make_interval(mins => COALESCE((pc3.collection_schedule->>'minute_offset')::int, 0)) > $1::timestamptz
		             THEN date_trunc('hour', $1::timestamptz)
		                  + make_interval(mins => COALESCE((pc3.collection_schedule->>'minute_offset')::int, 0))
		             ELSE date_trunc('hour', $1::timestamptz) + interval '1 hour'
		                  + make_interval(mins => COALESCE((pc3.collection_schedule->>'minute_offset')::int, 0))
		           END
		           FROM provider_configurations pc3
		           WHERE pc3.provider_id = p.id AND pc3.status = 'active'
		           ORDER BY pc3.created_at LIMIT 1)
		        ) AS next_scheduled
		 FROM (
		   SELECT DISTINCT provider_id, location_id FROM forecast_collections
		   UNION
		   SELECT DISTINCT pc.provider_id, cs.location_id
		   FROM collection_schedules cs
		   JOIN provider_configurations pc ON pc.id = cs.provider_configuration_id
		   WHERE cs.job_type = 'forecast_collection' AND cs.location_id IS NOT NULL
		 ) cell
		 JOIN providers p ON p.id = cell.provider_id
		 JOIN locations l ON l.id = cell.location_id
		 ORDER BY p.slug, l.name`,
		now.UTC())
	if err != nil {
		return nil, fmt.Errorf("health cells: %w", err)
	}
	defer rows.Close()
	var out []admin.CellHealth
	for rows.Next() {
		var c admin.CellHealth
		var lastStatus *string
		if err := rows.Scan(&c.ProviderID, &c.ProviderName, &c.ProviderSlug, &c.LocationID, &c.LocationName,
			&c.LastSuccessAt, &lastStatus, &c.NextScheduledAt); err != nil {
			return nil, fmt.Errorf("scan health cell: %w", err)
		}
		if lastStatus != nil {
			c.LastStatus = *lastStatus
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Circuits implements admin.HealthRepository (breaker state per provider that
// has a circuit row).
func (r *HealthRepository) Circuits(ctx context.Context, tx dbtx.DBTX) ([]admin.CircuitHealth, error) {
	rows, err := tx.Query(ctx,
		`SELECT pc.provider_id, p.name, pc.state, pc.consecutive_failures, pc.next_probe_at
		 FROM provider_circuits pc JOIN providers p ON p.id = pc.provider_id
		 ORDER BY p.slug`)
	if err != nil {
		return nil, fmt.Errorf("health circuits: %w", err)
	}
	defer rows.Close()
	var out []admin.CircuitHealth
	for rows.Next() {
		var c admin.CircuitHealth
		if err := rows.Scan(&c.ProviderID, &c.ProviderName, &c.State, &c.ConsecutiveFailures, &c.NextProbeAt); err != nil {
			return nil, fmt.Errorf("scan circuit health: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ObservationLocations implements admin.HealthRepository: per active location,
// the newest live observation and the 24 h suspect count.
func (r *HealthRepository) ObservationLocations(ctx context.Context, tx dbtx.DBTX, now time.Time) ([]admin.ObservationLocation, error) {
	rows, err := tx.Query(ctx,
		`SELECT l.id, l.name,
		        (SELECT max(o.observed_at) FROM observations o
		           WHERE o.location_id = l.id AND o.superseded_observation_id IS NULL) AS last_observed,
		        (SELECT count(*) FROM observations o
		           WHERE o.location_id = l.id AND o.quality_flag = 'suspect'
		             AND o.created_at > $1) AS suspect_24h
		 FROM locations l
		 WHERE l.status = 'active'
		 ORDER BY l.name`,
		now.Add(-24*time.Hour).UTC())
	if err != nil {
		return nil, fmt.Errorf("health observation locations: %w", err)
	}
	defer rows.Close()
	var out []admin.ObservationLocation
	for rows.Next() {
		var o admin.ObservationLocation
		if err := rows.Scan(&o.LocationID, &o.LocationName, &o.LastObservedAt, &o.SuspectCount24h); err != nil {
			return nil, fmt.Errorf("scan observation location: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// EngineLagSeconds implements admin.HealthRepository: now − the most recent live
// accuracy_metrics.calculated_at, or nil when no metrics exist yet.
func (r *HealthRepository) EngineLagSeconds(ctx context.Context, tx dbtx.DBTX, now time.Time) (*int64, error) {
	var latest *time.Time
	if err := tx.QueryRow(ctx,
		`SELECT max(calculated_at) FROM accuracy_metrics WHERE superseded_by IS NULL`).Scan(&latest); err != nil {
		return nil, fmt.Errorf("engine lag: %w", err)
	}
	if latest == nil {
		return nil, nil
	}
	lag := int64(now.UTC().Sub(latest.UTC()).Seconds())
	if lag < 0 {
		lag = 0
	}
	return &lag, nil
}
