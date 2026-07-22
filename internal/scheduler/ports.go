package scheduler

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// SlotRepository persists schedule slots. Implemented by
// adapters/persistence/schedulerpg.
type SlotRepository interface {
	// Generate inserts a slot if its uniqueness key is not already present
	// (idempotent slot generation; prevents double-generation).
	Generate(ctx context.Context, tx dbtx.DBTX, s *Slot) error
	// ClaimDue atomically claims up to limit due slots (and reclaims slots
	// whose lease expired) for instanceID, setting a fresh lease. Uses
	// FOR UPDATE SKIP LOCKED so concurrent workers never double-claim.
	ClaimDue(ctx context.Context, tx dbtx.DBTX, instanceID string, now time.Time, lease time.Duration, limit int) ([]*Slot, error)
	// Complete marks a slot completed and links its run.
	Complete(ctx context.Context, tx dbtx.DBTX, slotID, runID uuid.UUID) error
	// Fail records a failed attempt. When attempts < MaxAttempts the slot
	// returns to due with next_retry_at set (retry); otherwise it is failed.
	Fail(ctx context.Context, tx dbtx.DBTX, slotID, runID uuid.UUID, attempts int, nextRetryAt *time.Time) error
}

// RunRepository persists schedule run history.
type RunRepository interface {
	Start(ctx context.Context, tx dbtx.DBTX, r *Run) error
	Finish(ctx context.Context, tx dbtx.DBTX, runID uuid.UUID, status, errorCode, errorMessage string, durationMS, recordsAffected int) error
}

// Dispatcher executes the job a slot represents, returning the number of
// records affected (for run history). Implemented per job type; the slice
// provides a forecast-collection dispatcher.
type Dispatcher interface {
	Dispatch(ctx context.Context, slot *Slot) (recordsAffected int, err error)
}
