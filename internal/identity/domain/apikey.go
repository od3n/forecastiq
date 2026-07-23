package domain

import (
	"time"

	"github.com/google/uuid"
)

// APIKey is a personal access credential for programmatic API access. Only the
// argon2id hash of the secret is ever stored; the plaintext is shown once at
// creation and never again (WP-03 acceptance). KeyPrefix is the non-secret
// lookup handle carried in the presented key.
type APIKey struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	WorkspaceID     uuid.UUID
	Name            string
	KeyHash         string // argon2id encoded hash; never serialized to clients
	KeyPrefix       string // e.g. fiq_abc123 — lookup handle, not a secret
	Scopes          []string
	RateLimitPerMin int
	ExpiresAt       *time.Time
	CreatedAt       time.Time
	RevokedAt       *time.Time
	LastUsedAt      *time.Time
}

// Revoked reports whether the key has been revoked.
func (k *APIKey) Revoked() bool { return k.RevokedAt != nil }

// Expired reports whether the key's expiry has passed at now.
func (k *APIKey) Expired(now time.Time) bool {
	return k.ExpiresAt != nil && !now.Before(*k.ExpiresAt)
}

// Usable reports whether the key may authenticate a request at now (not
// revoked and not expired).
func (k *APIKey) Usable(now time.Time) bool {
	return !k.Revoked() && !k.Expired(now)
}

// DefaultScopes is the scope granted to a new key when none is requested.
func DefaultScopes() []string { return []string{"read:public"} }
