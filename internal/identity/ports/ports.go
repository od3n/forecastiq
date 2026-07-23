// Package ports declares the identity module's contracts: the token verifier
// (implemented by an auth adapter — Supabase JWKS in production, a dev verifier
// locally) and the persistence repositories (implemented by
// adapters/persistence/identitypg). Application services depend only on these
// interfaces (dependency rule: dependencies point inward).
package ports

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/identity/domain"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// Claims is the verified, provider-agnostic identity asserted by a bearer
// token. Only the subject mapping and email are consumed; the app role is
// always read from the database, never trusted from the token (security
// architecture §7).
type Claims struct {
	Subject   string // maps to users.auth_subject
	Email     string
	ExpiresAt time.Time
}

// TokenVerifier verifies a raw bearer token and returns its claims. It owns
// signature verification, key selection/rotation, and issuer/audience/expiry
// checks; it returns domain.ErrInvalidToken or domain.ErrTokenExpired on
// failure and never surfaces cryptographic internals.
type TokenVerifier interface {
	Verify(ctx context.Context, rawToken string) (*Claims, error)
}

// UserRepository persists User aggregates. Every method takes a dbtx.DBTX so it
// can run on the pool or inside a caller-owned transaction.
type UserRepository interface {
	GetByID(ctx context.Context, tx dbtx.DBTX, id uuid.UUID) (*domain.User, error)
	GetByAuthSubject(ctx context.Context, tx dbtx.DBTX, subject string) (*domain.User, error)
	Insert(ctx context.Context, tx dbtx.DBTX, u *domain.User) error
	// UpdateProfile persists the mutable profile fields (default location,
	// preferences). Identity fields (subject, email, role) are not mutated here.
	UpdateProfile(ctx context.Context, tx dbtx.DBTX, u *domain.User) error
	// TouchLastLogin records a successful authentication timestamp.
	TouchLastLogin(ctx context.Context, tx dbtx.DBTX, id uuid.UUID, at time.Time) error
}

// APIKeyRepository persists APIKey aggregates. List and lookup methods used for
// display MUST NOT populate KeyHash; GetByPrefix (authentication path) is the
// only method that returns the hash.
type APIKeyRepository interface {
	Insert(ctx context.Context, tx dbtx.DBTX, k *domain.APIKey) error
	// GetByPrefix returns the key (including KeyHash) for authentication, or
	// domain.ErrKeyNotFound.
	GetByPrefix(ctx context.Context, tx dbtx.DBTX, prefix string) (*domain.APIKey, error)
	// ListByUser returns a user's keys WITHOUT the hash (display/management).
	ListByUser(ctx context.Context, tx dbtx.DBTX, userID uuid.UUID) ([]*domain.APIKey, error)
	// GetByID returns a key WITHOUT the hash (ownership check on revoke).
	GetByID(ctx context.Context, tx dbtx.DBTX, id uuid.UUID) (*domain.APIKey, error)
	// Revoke sets revoked_at (idempotent; never reactivates).
	Revoke(ctx context.Context, tx dbtx.DBTX, id uuid.UUID, at time.Time) error
	TouchLastUsed(ctx context.Context, tx dbtx.DBTX, id uuid.UUID, at time.Time) error
}
