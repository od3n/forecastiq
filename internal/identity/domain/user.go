// Package domain holds the identity module's pure domain model: the User and
// APIKey entities, the Role value object, and domain errors. It imports only
// the standard library and the UUID kernel (ADR-022) — no infrastructure
// (binding rule: domain has zero infrastructure imports). Password hashing is
// never here: ADR-008 keeps credentials in Supabase; this module stores only
// the subject mapping and app role.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// Role is the application authorization role (distinct from the unauthenticated
// "public" access class, which is the absence of a user). The authoritative
// role always comes from the database, never from a JWT claim (security
// architecture §7).
type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

// Valid reports whether the role is a known value.
func (r Role) Valid() bool { return r == RoleUser || r == RoleAdmin }

// IsAdmin reports whether the role is the operator role.
func (r Role) IsAdmin() bool { return r == RoleAdmin }

// User maps a Supabase auth subject to an application account. It carries no
// password material (ADR-008). Status follows the shared entity lifecycle
// (active/disabled/archived) — a disabled user is denied authentication.
type User struct {
	ID                uuid.UUID
	WorkspaceID       uuid.UUID
	AuthSubject       string // Supabase user id (JWT `sub`)
	Email             string
	Role              Role
	Status            string // entity_status: active | disabled | archived
	DefaultLocationID *uuid.UUID
	Preferences       map[string]any
	CreatedAt         time.Time
	UpdatedAt         time.Time
	LastLoginAt       *time.Time
}

// Active reports whether the user may authenticate and act.
func (u *User) Active() bool { return u.Status == "active" }
