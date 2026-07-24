package identity

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/audit"
	"github.com/forecastiq/forecastiq/internal/identity/domain"
	"github.com/forecastiq/forecastiq/internal/identity/ports"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
	"github.com/forecastiq/forecastiq/internal/platform/ids"
)

const (
	// exportTTL is the download validity window for a completed export (AUTH-09).
	exportTTL = 24 * time.Hour
	// exportAuditLimit bounds the own-audit-events included in an export.
	exportAuditLimit = 1000
)

// ExportService implements the AUTH-09 GDPR account-data export. The export is
// generated inline (the payload is small — one user row + API-key metadata +
// own audit events), recorded as an export_jobs row (pending → completed so the
// one-active-per-user guard applies), and downloadable for 24h. Content is
// account data only; never a general report engine (reconciliation §2.2).
type ExportService struct {
	exports     ports.ExportJobRepository
	users       ports.UserRepository
	keys        ports.APIKeyRepository
	auditReader audit.Reader
	store       ports.ExportStore
	tx          *dbtx.Runner
	pool        dbtx.DBTX
	audit       audit.Recorder
	clock       clock.Clock
	logger      *slog.Logger
	workspaceID uuid.UUID
}

// NewExportService wires an ExportService.
func NewExportService(exports ports.ExportJobRepository, users ports.UserRepository,
	keys ports.APIKeyRepository, auditReader audit.Reader, store ports.ExportStore,
	tx *dbtx.Runner, pool dbtx.DBTX, rec audit.Recorder, clk clock.Clock,
	logger *slog.Logger, workspaceID uuid.UUID) *ExportService {
	return &ExportService{exports: exports, users: users, keys: keys, auditReader: auditReader,
		store: store, tx: tx, pool: pool, audit: rec, clock: clk, logger: logger, workspaceID: workspaceID}
}

// exportDoc is the account-data export payload (AUTH-09; account data only).
type exportDoc struct {
	ExportedAt  time.Time     `json:"exported_at"`
	User        exportUser    `json:"user"`
	APIKeys     []exportKey   `json:"api_keys"`
	AuditEvents []exportEvent `json:"audit_events"`
}

type exportUser struct {
	ID                string         `json:"id"`
	Email             string         `json:"email"`
	Role              string         `json:"role"`
	Status            string         `json:"status"`
	DefaultLocationID *string        `json:"default_location_id,omitempty"`
	Preferences       map[string]any `json:"preferences,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	LastLoginAt       *time.Time     `json:"last_login_at,omitempty"`
}

type exportKey struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	KeyPrefix  string     `json:"key_prefix"`
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

type exportEvent struct {
	ID           string         `json:"id"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	Details      map[string]any `json:"details,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

// exportKeyFor builds the scheme-prefixed object key for an export file.
func exportKeyFor(targetID, jobID uuid.UUID) string {
	return fmt.Sprintf("file://exports/%s/%s.json.gz", targetID, jobID)
}

// RequestExport creates and (inline) generates an account-data export for the
// target user. A pending row is inserted first so a concurrent request hits the
// one-active-per-user guard (409 via ErrExportInProgress); on generation
// failure the job is marked failed (freeing the guard).
func (s *ExportService) RequestExport(ctx context.Context, actor Principal, targetID uuid.UUID, ip string) (*domain.ExportJob, error) {
	target, err := s.users.GetByID(ctx, s.pool, targetID)
	if err != nil {
		return nil, err // ErrUserNotFound
	}
	now := s.clock.Now()
	selfService := targetID == actor.UserID
	tid := targetID
	job := &domain.ExportJob{
		ID: ids.New(), WorkspaceID: s.workspaceID, RequestedBy: actor.UserID,
		TargetUserID: &tid, Status: domain.ExportPending, CreatedAt: now,
	}
	action := "admin.export_requested"
	if selfService {
		action = "user.export_requested"
	}
	if terr := s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		if ierr := s.exports.Insert(ctx, tx, job); ierr != nil {
			return ierr // ErrExportInProgress
		}
		return s.audit.Record(ctx, tx, audit.Event{
			UserID: &actor.UserID, Action: action, ResourceType: "export_job",
			ResourceID: &job.ID, IPAddress: ip,
			Details: map[string]any{"target_user_id": targetID.String()}, At: now,
		})
	}); terr != nil {
		return nil, terr
	}

	// Generate the export inline, then mark the job completed.
	data, gerr := s.assemble(ctx, target)
	if gerr == nil {
		objectKey := exportKeyFor(targetID, job.ID)
		gerr = s.store.Write(ctx, objectKey, data)
		if gerr == nil {
			completedAt := s.clock.Now()
			expiresAt := completedAt.Add(exportTTL)
			cerr := s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
				if uerr := s.exports.Complete(ctx, tx, job.ID, objectKey, expiresAt, completedAt); uerr != nil {
					return uerr
				}
				return s.audit.Record(ctx, tx, audit.Event{
					UserID: &actor.UserID, Action: "user.export_completed", ResourceType: "export_job",
					ResourceID: &job.ID, IPAddress: ip,
					Details: map[string]any{"target_user_id": targetID.String()}, At: completedAt,
				})
			})
			if cerr == nil {
				job.Status = domain.ExportCompleted
				job.ObjectKey = objectKey
				job.ExpiresAt = &expiresAt
				job.CompletedAt = &completedAt
				return job, nil
			}
			gerr = cerr
		}
	}
	// Generation failed: mark the job failed (best-effort) and surface the error.
	_ = s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		return s.exports.Fail(ctx, tx, job.ID, "export generation failed")
	})
	s.logger.ErrorContext(ctx, "export.generation_failed",
		slog.String("job_id", job.ID.String()), slog.String("error", gerr.Error()))
	return nil, fmt.Errorf("generate export: %w", gerr)
}

// assemble builds the account-data JSON for the target user.
func (s *ExportService) assemble(ctx context.Context, target *domain.User) ([]byte, error) {
	keys, err := s.keys.ListByUser(ctx, s.pool, target.ID)
	if err != nil {
		return nil, err
	}
	events, _, err := s.auditReader.List(ctx, audit.Filter{UserID: &target.ID, Limit: exportAuditLimit})
	if err != nil {
		return nil, err
	}
	doc := exportDoc{ExportedAt: s.clock.Now().UTC(), User: exportUserFrom(target)}
	for _, k := range keys {
		doc.APIKeys = append(doc.APIKeys, exportKey{
			ID: k.ID.String(), Name: k.Name, KeyPrefix: k.KeyPrefix, Scopes: k.Scopes,
			CreatedAt: k.CreatedAt.UTC(), RevokedAt: k.RevokedAt, LastUsedAt: k.LastUsedAt,
		})
	}
	for _, e := range events {
		doc.AuditEvents = append(doc.AuditEvents, exportEvent{
			ID: e.ID.String(), Action: e.Action, ResourceType: e.ResourceType,
			Details: e.Details, CreatedAt: e.CreatedAt.UTC(),
		})
	}
	return json.MarshalIndent(doc, "", "  ")
}

func exportUserFrom(u *domain.User) exportUser {
	eu := exportUser{
		ID: u.ID.String(), Email: u.Email, Role: string(u.Role), Status: u.Status,
		Preferences: u.Preferences, CreatedAt: u.CreatedAt.UTC(), LastLoginAt: u.LastLoginAt,
	}
	if u.DefaultLocationID != nil {
		s := u.DefaultLocationID.String()
		eu.DefaultLocationID = &s
	}
	return eu
}

// DownloadExport returns the export file bytes for a completed, unexpired job
// the actor may access: the requester, the target, or an admin (object-level
// rule §4). Any other case returns ErrExportNotFound (no existence disclosure).
func (s *ExportService) DownloadExport(ctx context.Context, actor Principal, jobID uuid.UUID) ([]byte, error) {
	job, err := s.exports.GetByID(ctx, s.pool, jobID)
	if err != nil {
		return nil, err // ErrExportNotFound
	}
	authorized := job.RequestedBy == actor.UserID ||
		(job.TargetUserID != nil && *job.TargetUserID == actor.UserID) ||
		actor.IsAdmin()
	if !authorized {
		return nil, domain.ErrExportNotFound
	}
	if job.Status != domain.ExportCompleted || job.ObjectKey == "" {
		return nil, domain.ErrExportNotFound
	}
	if job.Expired(s.clock.Now()) {
		return nil, domain.ErrExportExpired
	}
	return s.store.Read(ctx, job.ObjectKey)
}
