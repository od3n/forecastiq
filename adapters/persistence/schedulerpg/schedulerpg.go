// Package schedulerpg implements the scheduler module's repository ports on
// PostgreSQL. Slot claiming uses FOR UPDATE SKIP LOCKED so concurrent workers
// never double-claim (ADR-005; QX-05). Parameterized queries throughout.
package schedulerpg

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
	"github.com/forecastiq/forecastiq/internal/scheduler"
)

// SlotRepository implements scheduler.SlotRepository.
type SlotRepository struct{}

// NewSlotRepository returns a SlotRepository.
func NewSlotRepository() *SlotRepository { return &SlotRepository{} }

// slotColumnsQualified qualifies every column with the table alias s (needed
// in the claim CTE join, where the claimable CTE also exposes an id column).
const slotColumnsQualified = `s.id, s.provider_configuration_id, s.job_type, s.location_id, s.slot_time, s.status,
	COALESCE(s.claimed_by,''), s.claimed_at, s.lease_expires_at, s.attempts, s.next_retry_at,
	s.schedule_run_id, s.created_at, s.updated_at`

func scanSlot(row pgx.Row) (*scheduler.Slot, error) {
	var s scheduler.Slot
	err := row.Scan(&s.ID, &s.ProviderConfigurationID, &s.JobType, &s.LocationID, &s.SlotTime, &s.Status,
		&s.ClaimedBy, &s.ClaimedAt, &s.LeaseExpiresAt, &s.Attempts, &s.NextRetryAt,
		&s.ScheduleRunID, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("scan slot: %w", err)
	}
	return &s, nil
}

// Generate implements scheduler.SlotRepository (idempotent slot creation).
func (r *SlotRepository) Generate(ctx context.Context, tx dbtx.DBTX, s *scheduler.Slot) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO collection_schedules
		   (id, provider_configuration_id, job_type, location_id, slot_time, status, attempts, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,0,$7,$8)
		 ON CONFLICT DO NOTHING`,
		s.ID, s.ProviderConfigurationID, s.JobType, s.LocationID, s.SlotTime, s.Status, s.CreatedAt, s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("generate slot: %w", err)
	}
	return nil
}

// ClaimDue implements scheduler.SlotRepository: atomically claims due slots
// (and reclaims slots with expired leases) using FOR UPDATE SKIP LOCKED.
func (r *SlotRepository) ClaimDue(ctx context.Context, tx dbtx.DBTX, instanceID string, now time.Time, lease time.Duration, limit int) ([]*scheduler.Slot, error) {
	leaseExpiry := now.Add(lease)
	rows, err := tx.Query(ctx,
		`WITH claimable AS (
		   SELECT id FROM collection_schedules
		   WHERE (status = 'due' AND slot_time <= $1 AND (next_retry_at IS NULL OR next_retry_at <= $1))
		      OR (status = 'claimed' AND lease_expires_at < $1)
		   ORDER BY slot_time
		   LIMIT $2
		   FOR UPDATE SKIP LOCKED
		 )
		 UPDATE collection_schedules s
		 SET status = 'claimed', claimed_by = $3, claimed_at = $1, lease_expires_at = $4
		 FROM claimable c WHERE s.id = c.id
		 RETURNING `+slotColumnsQualified, now, limit, instanceID, leaseExpiry)
	if err != nil {
		return nil, fmt.Errorf("claim slots: %w", err)
	}
	defer rows.Close()
	var out []*scheduler.Slot
	for rows.Next() {
		s, err := scanSlot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Complete implements scheduler.SlotRepository.
func (r *SlotRepository) Complete(ctx context.Context, tx dbtx.DBTX, slotID, runID uuid.UUID) error {
	_, err := tx.Exec(ctx,
		`UPDATE collection_schedules SET status = 'completed', schedule_run_id = $2 WHERE id = $1`,
		slotID, runID)
	if err != nil {
		return fmt.Errorf("complete slot: %w", err)
	}
	return nil
}

// Fail implements scheduler.SlotRepository: returns the slot to due (with a
// retry time) while attempts remain, otherwise marks it failed.
func (r *SlotRepository) Fail(ctx context.Context, tx dbtx.DBTX, slotID, runID uuid.UUID, attempts int, nextRetryAt *time.Time) error {
	_, err := tx.Exec(ctx,
		`UPDATE collection_schedules SET
		   status = CASE WHEN $4::timestamptz IS NOT NULL THEN 'due' ELSE 'failed' END,
		   attempts = $3, next_retry_at = $4, schedule_run_id = $2,
		   claimed_by = NULL, claimed_at = NULL, lease_expires_at = NULL
		 WHERE id = $1`,
		slotID, runID, attempts, nextRetryAt)
	if err != nil {
		return fmt.Errorf("fail slot: %w", err)
	}
	return nil
}

// CountClaimable implements scheduler.SlotRepository: counts slots eligible
// for claiming now (mirrors the ClaimDue predicate without locking).
func (r *SlotRepository) CountClaimable(ctx context.Context, tx dbtx.DBTX, now time.Time) (int, error) {
	var n int
	err := tx.QueryRow(ctx,
		`SELECT count(*) FROM collection_schedules
		 WHERE (status = 'due' AND slot_time <= $1 AND (next_retry_at IS NULL OR next_retry_at <= $1))
		    OR (status = 'claimed' AND lease_expires_at < $1)`, now).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count claimable slots: %w", err)
	}
	return n, nil
}

// RunRepository implements scheduler.RunRepository.
type RunRepository struct{}

// NewRunRepository returns a RunRepository.
func NewRunRepository() *RunRepository { return &RunRepository{} }

// Start implements scheduler.RunRepository.
func (r *RunRepository) Start(ctx context.Context, tx dbtx.DBTX, run *scheduler.Run) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO schedule_runs (id, job_type, slot_id, started_at, status, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		run.ID, run.JobType, run.SlotID, run.StartedAt, run.Status, run.CreatedAt)
	if err != nil {
		return fmt.Errorf("start run: %w", err)
	}
	return nil
}

// Finish implements scheduler.RunRepository.
func (r *RunRepository) Finish(ctx context.Context, tx dbtx.DBTX, runID uuid.UUID, status, errorCode, errorMessage string, durationMS, recordsAffected int) error {
	_, err := tx.Exec(ctx,
		`UPDATE schedule_runs SET
		   completed_at = now(), status = $2, error_code = $3, error_message = $4,
		   duration_ms = $5, records_affected = $6
		 WHERE id = $1`,
		runID, status, errorCode, errorMessage, durationMS, recordsAffected)
	if err != nil {
		return fmt.Errorf("finish run: %w", err)
	}
	return nil
}
