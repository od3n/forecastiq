package respond

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Gin context keys for request-scoped values.
const (
	RequestIDKey = "fiq.request_id"
	PrincipalKey = "fiq.principal"
)

// Principal is the authenticated caller (populated by the auth middleware).
// Until WP-03/19 lands Supabase JWKS auth, the dev-token seam yields an admin
// principal with no UserID.
type Principal struct {
	UserID *uuid.UUID
	Role   string // admin | user
	Name   string
}

// IsAdmin reports whether the principal has the admin role.
func (p Principal) IsAdmin() bool { return p.Role == "admin" }

// SetRequestID stores the request id in the gin context.
func SetRequestID(c *gin.Context, id string) { c.Set(RequestIDKey, id) }

// RequestID returns the request id from the gin context (empty if unset).
func RequestID(c *gin.Context) string {
	if v, ok := c.Get(RequestIDKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// SetPrincipal stores the authenticated principal.
func SetPrincipal(c *gin.Context, p Principal) { c.Set(PrincipalKey, p) }

// PrincipalFrom returns the principal from the gin context.
func PrincipalFrom(c *gin.Context) (Principal, bool) {
	if v, ok := c.Get(PrincipalKey); ok {
		if p, ok := v.(Principal); ok {
			return p, true
		}
	}
	return Principal{}, false
}
