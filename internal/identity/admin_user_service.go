package identity

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/audit"
	"github.com/forecastiq/forecastiq/internal/identity/domain"
	"github.com/forecastiq/forecastiq/internal/identity/ports"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// AdminUserService implements the S-14 admin user-lifecycle operations
// (ADMIN-05 / AUTH-09): list, disable/enable, and delete — each audited and
// propagated to the managed auth provider (ADR-008 §6). The local users table
// is authoritative; propagation ordering follows docs/api/07 §6 (ban before the
// local disable so a propagation failure leaves the row unchanged; delete after
// the local delete commits, best-effort).
type AdminUserService struct {
	users    ports.UserRepository
	supabase ports.SupabaseAdmin
	tx       *dbtx.Runner
	pool     dbtx.DBTX
	audit    audit.Recorder
	clock    clock.Clock
	logger   *slog.Logger
}

// NewAdminUserService wires an AdminUserService.
func NewAdminUserService(users ports.UserRepository, supabase ports.SupabaseAdmin, tx *dbtx.Runner,
	pool dbtx.DBTX, rec audit.Recorder, clk clock.Clock, logger *slog.Logger) *AdminUserService {
	return &AdminUserService{users: users, supabase: supabase, tx: tx, pool: pool, audit: rec, clock: clk, logger: logger}
}

// statusError is a field-validation error surfaced as 422 by the API layer.
type statusError struct{ msg string }

func (e *statusError) Error() string   { return e.msg }
func (e *statusError) Field() string   { return "status" }
func (e *statusError) Message() string { return e.msg }

// List returns a page of users for the admin surface (S-14).
func (s *AdminUserService) List(ctx context.Context, limit int, cursor uuid.UUID) ([]*domain.User, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.users.List(ctx, s.pool, limit, cursor)
}

// SetStatus disables or enables a target user (admin). A self-target is refused
// (409 self-lockout). The change is propagated to the auth provider first
// (ban/unban), so a propagation failure leaves the local row unchanged
// (docs/api/07 §6), then persisted with an audit record in one transaction.
func (s *AdminUserService) SetStatus(ctx context.Context, actor Principal, targetID uuid.UUID, status, ip string) (*domain.User, error) {
	if status != "active" && status != "disabled" {
		return nil, &statusError{msg: "status must be one of active|disabled"}
	}
	if targetID == actor.UserID {
		return nil, domain.ErrSelfLockout
	}
	target, err := s.users.GetByID(ctx, s.pool, targetID)
	if err != nil {
		return nil, err // ErrUserNotFound
	}
	banned := status == "disabled"
	if perr := s.supabase.SetBanned(ctx, target.AuthSubject, banned); perr != nil {
		s.logger.ErrorContext(ctx, "supabase.set_banned_failed",
			slog.String("target", targetID.String()), slog.String("error", perr.Error()))
		return nil, domain.ErrUpstreamAuth
	}
	action := "admin.user_enabled"
	if banned {
		action = "admin.user_disabled"
	}
	now := s.clock.Now()
	if terr := s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		if uerr := s.users.UpdateStatus(ctx, tx, targetID, status); uerr != nil {
			return uerr
		}
		return s.audit.Record(ctx, tx, audit.Event{
			UserID: &actor.UserID, Action: action, ResourceType: "user",
			ResourceID: &targetID, IPAddress: ip,
			Details: map[string]any{"target_user_id": targetID.String(), "status": status}, At: now,
		})
	}); terr != nil {
		return nil, terr
	}
	target.Status = status
	return target, nil
}

// Delete removes a user account (AUTH-09). selfService is true for DELETE /me
// (self-delete is allowed); the admin surface passes false and a self-target is
// refused (409 self-lockout). The local delete + audit run in one transaction
// under the immutable-write GUC (so the audit FK anonymizes the actor rather
// than blocking the delete); the auth-provider delete is best-effort afterwards.
func (s *AdminUserService) Delete(ctx context.Context, actor Principal, targetID uuid.UUID, selfService bool, ip string) error {
	if !selfService && targetID == actor.UserID {
		return domain.ErrSelfLockout
	}
	target, err := s.users.GetByID(ctx, s.pool, targetID)
	if err != nil {
		return err
	}
	action := "admin.user_deleted"
	if selfService {
		action = "user.deleted"
	}
	now := s.clock.Now()
	if terr := s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		// Permit the FK-driven audit anonymization for this transaction only.
		if _, eerr := tx.Exec(ctx, `SET LOCAL app.allow_immutable_write = 'on'`); eerr != nil {
			return fmt.Errorf("enable immutable write: %w", eerr)
		}
		if rerr := s.audit.Record(ctx, tx, audit.Event{
			UserID: &actor.UserID, Action: action, ResourceType: "user",
			ResourceID: &targetID, IPAddress: ip,
			Details: map[string]any{"target_user_id": targetID.String(), "self_service": selfService}, At: now,
		}); rerr != nil {
			return rerr
		}
		return s.users.Delete(ctx, tx, targetID)
	}); terr != nil {
		return terr
	}
	if perr := s.supabase.DeleteUser(ctx, target.AuthSubject); perr != nil {
		s.logger.ErrorContext(ctx, "supabase.delete_user_failed",
			slog.String("target", targetID.String()), slog.String("error", perr.Error()))
	}
	return nil
}
