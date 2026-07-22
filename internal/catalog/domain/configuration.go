package domain

import (
	"time"

	"github.com/google/uuid"
)

// ProviderConfiguration is the per-(workspace, provider) operational config:
// credential reference, collection schedule, adapter version, validation
// state. The credential_ref names an environment variable (BR-08); the
// secret value itself never lives in the database.
type ProviderConfiguration struct {
	ID                 uuid.UUID
	WorkspaceID        uuid.UUID
	ProviderID         uuid.UUID
	Status             Status
	CredentialRef      string // env var name; empty when the provider needs no key
	CollectionSchedule Schedule
	AdapterVersion     string
	ValidationState    string // unvalidated | validated | failed
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// Schedule is the cron-like collection cadence (JSONB in storage). The MVP
// supports an hourly interval with a minute offset.
type Schedule struct {
	Interval     string `json:"interval"`      // "hourly"
	MinuteOffset int    `json:"minute_offset"` // 0..59
}

// DefaultSchedule is the standard hourly cadence.
func DefaultSchedule() Schedule { return Schedule{Interval: "hourly", MinuteOffset: 0} }

// Valid reports whether the schedule is a supported cadence.
func (s Schedule) Valid() bool {
	return s.Interval == "hourly" && s.MinuteOffset >= 0 && s.MinuteOffset <= 59
}

// SlotTimes enumerates the scheduled instants in the half-open window
// [from, to) for this cadence. For the hourly interval these are the top of
// each hour shifted by MinuteOffset. Pure and deterministic (used by the
// scheduler to generate due slots).
func (s Schedule) SlotTimes(from, to time.Time) []time.Time {
	if !s.Valid() || !to.After(from) {
		return nil
	}
	// Align to the hour containing `from`, then apply the minute offset.
	cursor := from.UTC().Truncate(time.Hour).Add(time.Duration(s.MinuteOffset) * time.Minute)
	if cursor.Before(from) {
		cursor = cursor.Add(time.Hour)
	}
	var out []time.Time
	for cursor.Before(to) {
		out = append(out, cursor)
		cursor = cursor.Add(time.Hour)
	}
	return out
}
