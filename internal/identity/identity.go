// Package identity is the identity module's application layer (ADR-008): it
// verifies Supabase-issued JWTs, provisions users on first authenticated use,
// manages personal API keys (argon2id, shown once), and reads the current
// user's profile. HTTP handlers and route wiring land in WP-15/WP-19; this
// package delivers the use cases and their tests. The authoritative role is
// always read from the database, never trusted from a token claim.
package identity

import (
	"time"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/identity/domain"
)

// Re-exported domain aliases so consuming code references identity entities
// through the module's public surface.
type (
	User   = domain.User
	APIKey = domain.APIKey
	Role   = domain.Role
	// ExportJob is the GDPR account-data export record (WP-19c).
	ExportJob = domain.ExportJob
)

// Role values re-exported for consumer convenience.
const (
	RoleUser  = domain.RoleUser
	RoleAdmin = domain.RoleAdmin
)

// AuthMethod records how a principal authenticated (for audit + rate policy).
type AuthMethod string

const (
	AuthJWT    AuthMethod = "jwt"
	AuthAPIKey AuthMethod = "api_key"
)

// Principal is the authenticated caller resolved by the identity module: the
// database-backed identity (never the raw token). The API layer maps this onto
// its request-scoped principal (WP-19).
type Principal struct {
	UserID      uuid.UUID
	WorkspaceID uuid.UUID
	Email       string
	Role        Role
	Method      AuthMethod
	Scopes      []string // API-key scopes; empty for JWT (full user rights)
}

// IsAdmin reports whether the principal holds the admin role.
func (p Principal) IsAdmin() bool { return p.Role.IsAdmin() }

// Actor carries request context for audit attribution.
type Actor struct {
	IPAddress string
}

// UpdateProfileInput is the PATCH /me command (mutable profile fields only).
// Nil fields are left unchanged.
type UpdateProfileInput struct {
	DefaultLocationID *uuid.UUID
	Preferences       map[string]any
	Actor             Actor
}

// CreateKeyInput is the create-API-key command.
type CreateKeyInput struct {
	Name            string
	Scopes          []string
	RateLimitPerMin int
	ExpiresAt       *time.Time
	Actor           Actor
}

// CreatedKey bundles a newly created key's metadata with its plaintext secret.
// The plaintext is returned exactly once (never persisted, never re-derivable);
// callers must surface it to the user immediately and then discard it.
type CreatedKey struct {
	Key       *APIKey
	Plaintext string
}
