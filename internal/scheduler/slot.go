// Package scheduler implements the in-process, DB-backed scheduler (ADR-005):
// slot generation from active configurations, due-slot claiming via
// FOR UPDATE SKIP LOCKED, leases, dispatch to collection, run history, and
// retry. Multi-instance-safe from day one (slots are the coordination point).
package scheduler

import (
	"time"

	"github.com/google/uuid"
)

// Job types dispatched by the scheduler.
const (
	JobForecastCollection    = "forecast_collection"
	JobObservationCollection = "observation_collection"
)

// Slot statuses (collection_schedules.status).
const (
	SlotDue       = "due"
	SlotClaimed   = "claimed"
	SlotCompleted = "completed"
	SlotFailed    = "failed"
	SlotExpired   = "expired"
)

// Run statuses (schedule_runs.status).
const (
	RunRunning   = "running"
	RunCompleted = "completed"
	RunFailed    = "failed"
)

// MaxAttempts is the per-slot retry ceiling (FC-08).
const MaxAttempts = 5

// Slot is one scheduled unit of work (collection_schedules row).
type Slot struct {
	ID                      uuid.UUID
	ProviderConfigurationID uuid.UUID
	JobType                 string
	LocationID              *uuid.UUID
	SlotTime                time.Time
	Status                  string
	ClaimedBy               string
	ClaimedAt               *time.Time
	LeaseExpiresAt          *time.Time
	Attempts                int
	NextRetryAt             *time.Time
	ScheduleRunID           *uuid.UUID
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// Run is one job-execution history row (schedule_runs).
type Run struct {
	ID              uuid.UUID
	JobType         string
	SlotID          *uuid.UUID
	StartedAt       time.Time
	CompletedAt     *time.Time
	Status          string
	ErrorCode       string
	ErrorMessage    string
	DurationMS      int
	RecordsAffected int
	CreatedAt       time.Time
}
