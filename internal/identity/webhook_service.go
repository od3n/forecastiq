package identity

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/audit"
	"github.com/forecastiq/forecastiq/internal/identity/ports"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// WebhookEvent is a verified auth event delivered by the managed auth provider
// (Supabase). Signature verification happens at the HTTP edge; this carries the
// already-trusted fields.
type WebhookEvent struct {
	Type    string // provider event type (mapped to an audit action)
	Subject string // users.auth_subject (best-effort resolution to a user id)
	IP      string
}

// webhookActions maps provider event types to the stable audit action registry
// (audit-requirements §2). Unlisted types are ignored.
var webhookActions = map[string]string{
	"login":                       "auth.login",
	"signed_in":                   "auth.login",
	"login_failed":                "auth.login_failed",
	"signup":                      "auth.registered",
	"user.created":                "auth.registered",
	"email_verified":              "auth.email_verified",
	"password_recovery_requested": "auth.password_reset_requested",
	"password_recovery_completed": "auth.password_reset_completed",
	"logout":                      "auth.logout",
	"signed_out":                  "auth.logout",
}

// WebhookService ingests signed auth-provider webhook events into the audit
// trail (audit-requirements §5). It is best-effort by contract: the HTTP
// receiver acknowledges the provider regardless of the outcome here, so the
// auth flow never depends on our audit availability.
type WebhookService struct {
	users  ports.UserRepository
	tx     *dbtx.Runner
	pool   dbtx.DBTX
	audit  audit.Recorder
	clock  clock.Clock
	logger *slog.Logger
}

// NewWebhookService wires a WebhookService.
func NewWebhookService(users ports.UserRepository, tx *dbtx.Runner, pool dbtx.DBTX,
	rec audit.Recorder, clk clock.Clock, logger *slog.Logger) *WebhookService {
	return &WebhookService{users: users, tx: tx, pool: pool, audit: rec, clock: clk, logger: logger}
}

// Ingest records an audit row for a recognized event type. Unrecognized types
// are ignored (returns nil). The auth_subject is resolved to a user id
// best-effort; details carry the subject reference only (never the email;
// sanitization §3).
func (s *WebhookService) Ingest(ctx context.Context, ev WebhookEvent) error {
	action, ok := webhookActions[ev.Type]
	if !ok {
		s.logger.DebugContext(ctx, "webhook.ignored_event", slog.String("type", ev.Type))
		return nil
	}
	var userID *uuid.UUID
	if ev.Subject != "" {
		if u, err := s.users.GetByAuthSubject(ctx, s.pool, ev.Subject); err == nil {
			userID = &u.ID
		}
	}
	now := s.clock.Now()
	return s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		return s.audit.Record(ctx, tx, audit.Event{
			UserID: userID, Action: action, ResourceType: "auth",
			IPAddress: ev.IP,
			Details:   map[string]any{"event_type": ev.Type, "auth_subject": ev.Subject},
			At:        now,
		})
	})
}
