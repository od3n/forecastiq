package domain

import (
	"time"

	"github.com/google/uuid"
)

// Export job statuses (export_jobs.status).
const (
	ExportPending   = "pending"
	ExportCompleted = "completed"
	ExportFailed    = "failed"
)

// ExportJob is the GDPR account-data export record (AUTH-09; reconciliation
// §2.2). It is ownership-bearing (WorkspaceID) and scoped to account data only
// (user row + API-key metadata + own audit events) — never a general report
// engine. The export file lives on the payload volume, addressed by ObjectKey,
// and is downloadable until ExpiresAt (24h).
type ExportJob struct {
	ID           uuid.UUID
	WorkspaceID  uuid.UUID
	RequestedBy  uuid.UUID
	TargetUserID *uuid.UUID
	Status       string
	ObjectKey    string
	ExpiresAt    *time.Time
	CompletedAt  *time.Time
	ErrorMessage string
	CreatedAt    time.Time
}

// Expired reports whether the export's download window has closed at now.
func (j *ExportJob) Expired(now time.Time) bool {
	return j.ExpiresAt != nil && !now.Before(*j.ExpiresAt)
}
