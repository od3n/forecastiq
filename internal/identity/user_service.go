package identity

import (
	"context"
	"errors"
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

// UserService verifies tokens, provisions users on first use, and serves the
// current-user profile (module architecture §3.2). Provisioning and login
// happen in one bounded transaction with their audit record (ADR-027).
type UserService struct {
	users       ports.UserRepository
	verifier    ports.TokenVerifier
	tx          *dbtx.Runner
	pool        dbtx.DBTX
	audit       audit.Recorder
	clock       clock.Clock
	logger      *slog.Logger
	workspaceID uuid.UUID // system workspace for provisioning (single-workspace MVP, ADR-009)
}

// NewUserService wires a UserService. workspaceID is the workspace new users
// are provisioned into.
func NewUserService(users ports.UserRepository, verifier ports.TokenVerifier, tx *dbtx.Runner,
	pool dbtx.DBTX, rec audit.Recorder, clk clock.Clock, logger *slog.Logger, workspaceID uuid.UUID) *UserService {
	return &UserService{users: users, verifier: verifier, tx: tx, pool: pool,
		audit: rec, clock: clk, logger: logger, workspaceID: workspaceID}
}

// Authenticate verifies a bearer JWT, provisions the user on first use, records
// the login, and returns the database-backed principal. The role is read from
// the database, never from the token.
func (s *UserService) Authenticate(ctx context.Context, rawToken string, actor Actor) (*Principal, error) {
	claims, err := s.verifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, err // domain.ErrInvalidToken / ErrTokenExpired
	}
	now := s.clock.Now()

	user, err := s.resolveOrProvision(ctx, claims, now)
	if err != nil {
		return nil, err
	}
	if !user.Active() {
		return nil, domain.ErrUserDisabled
	}

	if terr := s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		return s.users.TouchLastLogin(ctx, tx, user.ID, now)
	}); terr != nil {
		return nil, fmt.Errorf("record login: %w", terr)
	}

	return &Principal{
		UserID: user.ID, WorkspaceID: user.WorkspaceID, Email: user.Email,
		Role: user.Role, Method: AuthJWT,
	}, nil
}

// resolveOrProvision returns the user for the claims' subject, provisioning a
// new active user on first authenticated use (idempotent under concurrency:
// a losing insert race re-reads the winner).
func (s *UserService) resolveOrProvision(ctx context.Context, claims *ports.Claims, now time.Time) (*domain.User, error) {
	existing, err := s.users.GetByAuthSubject(ctx, s.pool, claims.Subject)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, domain.ErrUserNotFound) {
		return nil, fmt.Errorf("lookup user: %w", err)
	}

	user := &domain.User{
		ID: ids.New(), WorkspaceID: s.workspaceID,
		AuthSubject: claims.Subject, Email: claims.Email,
		Role: domain.RoleUser, Status: "active",
		Preferences: map[string]any{}, CreatedAt: now, UpdatedAt: now,
	}
	perr := s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		if ierr := s.users.Insert(ctx, tx, user); ierr != nil {
			return ierr
		}
		return s.audit.Record(ctx, tx, audit.Event{
			UserID: &user.ID, Action: "user.provisioned", ResourceType: "user",
			ResourceID: &user.ID, IPAddress: "",
			Details: map[string]any{"email": user.Email, "auth_subject": user.AuthSubject},
			At:      now,
		})
	})
	if perr == nil {
		s.logger.InfoContext(ctx, "user.provisioned", slog.String("user_id", user.ID.String()))
		return user, nil
	}
	// A concurrent request provisioned the same subject first — re-read it.
	if errors.Is(perr, domain.ErrDuplicateSubject) {
		return s.users.GetByAuthSubject(ctx, s.pool, claims.Subject)
	}
	return nil, fmt.Errorf("provision user: %w", perr)
}

// GetMe returns the current user's profile.
func (s *UserService) GetMe(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	return s.users.GetByID(ctx, s.pool, userID)
}

// UpdateMe updates the current user's mutable profile fields (default location,
// preferences) and audits the change. Identity fields are never mutated here.
func (s *UserService) UpdateMe(ctx context.Context, userID uuid.UUID, in UpdateProfileInput) (*domain.User, error) {
	var updated *domain.User
	err := s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		user, gerr := s.users.GetByID(ctx, tx, userID)
		if gerr != nil {
			return gerr
		}
		if in.DefaultLocationID != nil {
			user.DefaultLocationID = in.DefaultLocationID
		}
		if in.Preferences != nil {
			user.Preferences = in.Preferences
		}
		user.UpdatedAt = s.clock.Now()
		if uerr := s.users.UpdateProfile(ctx, tx, user); uerr != nil {
			return uerr
		}
		updated = user
		return s.audit.Record(ctx, tx, audit.Event{
			UserID: &userID, Action: "user.update_profile", ResourceType: "user",
			ResourceID: &userID, IPAddress: in.Actor.IPAddress,
			Details: map[string]any{"fields": profileFields(in)}, At: s.clock.Now(),
		})
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// profileFields lists which mutable fields a profile update touched (for audit;
// never records the values, only the field names).
func profileFields(in UpdateProfileInput) []string {
	var f []string
	if in.DefaultLocationID != nil {
		f = append(f, "default_location_id")
	}
	if in.Preferences != nil {
		f = append(f, "preferences")
	}
	return f
}
