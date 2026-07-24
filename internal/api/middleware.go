// Package api is the HTTP layer: routing, middleware (request id, logging,
// recovery, metrics, auth, rate limit, CORS), and thin handlers that call
// module use cases and assemble envelopes (module architecture §3.5). Handlers
// contain no business logic.
package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/api/respond"
	"github.com/forecastiq/forecastiq/internal/identity"
	"github.com/forecastiq/forecastiq/internal/platform/metrics"
	"github.com/forecastiq/forecastiq/internal/platform/ratelimit"
)

// RequestID validates the inbound X-Request-Id (UUID) or generates one, and
// echoes it in the response (correlation surface; NFR-OBS02).
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-Id")
		if _, err := uuid.Parse(id); err != nil {
			id = uuid.NewString()
		}
		respond.SetRequestID(c, id)
		c.Header("X-Request-Id", id)
		c.Next()
	}
}

// RequestLogger emits the RED summary log line per request.
func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.InfoContext(c.Request.Context(), "api.request",
			slog.String("request_id", respond.RequestID(c)),
			slog.String("method", c.Request.Method),
			slog.String("route", c.FullPath()),
			slog.Int("status", c.Writer.Status()),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		)
	}
}

// Recovery converts panics into a sanitized 500 problem (no stack/SQL leak).
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if p := recover(); p != nil {
				logger.ErrorContext(c.Request.Context(), "api.panic",
					slog.String("request_id", respond.RequestID(c)),
					slog.Any("panic", p))
				respond.Error(c, errors.New("internal"), respond.RequestID(c), c.Request.URL.Path)
			}
		}()
		c.Next()
	}
}

// Metrics records HTTP RED metrics keyed by route template.
func Metrics(m *metrics.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		statusClass := strconv.Itoa(c.Writer.Status()/100) + "xx"
		m.HTTPRequestsTotal.WithLabelValues(c.Request.Method, route, statusClass).Inc()
		m.HTTPRequestDuration.WithLabelValues(c.Request.Method, route).Observe(time.Since(start).Seconds())
	}
}

// UserAuthenticator verifies a bearer JWT and resolves the database-backed
// principal (provisioning on first use). Implemented by *identity.UserService.
type UserAuthenticator interface {
	Authenticate(ctx context.Context, rawToken string, actor identity.Actor) (*identity.Principal, error)
}

// KeyAuthenticator resolves a presented X-API-Key to its principal (with the
// key's scopes). Implemented by *identity.APIKeyService.
type KeyAuthenticator interface {
	AuthenticateAPIKey(ctx context.Context, rawKey string) (*identity.Principal, error)
}

// Auth bundles the identity authenticators the middleware chain uses. It is the
// single seam through which the HTTP layer verifies callers (ADR-008/ADR-017):
// an X-API-Key takes precedence, otherwise a Bearer JWT is verified.
type Auth struct {
	Users UserAuthenticator
	Keys  KeyAuthenticator
}

// principalFromIdentity maps the module-resolved principal onto the request
// principal. Name defaults to the email for audit labelling.
func principalFromIdentity(ip *identity.Principal) respond.Principal {
	uid, wid := ip.UserID, ip.WorkspaceID
	return respond.Principal{
		UserID: &uid, WorkspaceID: &wid, Email: ip.Email,
		Role: string(ip.Role), Name: ip.Email,
		Method: string(ip.Method), Scopes: ip.Scopes,
	}
}

// resolve authenticates the caller. An X-API-Key is tried first; otherwise a
// Bearer JWT. Any verification failure (invalid/expired token, unusable key,
// disabled user) returns ok=false so the caller emits a uniform 401 with no
// oracle about which factor failed.
func (a Auth) resolve(c *gin.Context) (respond.Principal, bool) {
	ctx := c.Request.Context()
	if key := strings.TrimSpace(c.GetHeader("X-API-Key")); key != "" {
		if a.Keys == nil {
			return respond.Principal{}, false
		}
		p, err := a.Keys.AuthenticateAPIKey(ctx, key)
		if err != nil {
			return respond.Principal{}, false
		}
		return principalFromIdentity(p), true
	}
	if h := c.GetHeader("Authorization"); strings.HasPrefix(h, "Bearer ") {
		token := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
		if token == "" || a.Users == nil {
			return respond.Principal{}, false
		}
		p, err := a.Users.Authenticate(ctx, token, identity.Actor{IPAddress: c.ClientIP()})
		if err != nil {
			return respond.Principal{}, false
		}
		return principalFromIdentity(p), true
	}
	return respond.Principal{}, false
}

// RequireAuth gates authenticated routes: a valid JWT or API key resolving to
// an active user. The resolved principal is placed on the context for
// downstream role/scope middleware and handlers.
func (a Auth) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := a.resolve(c)
		if !ok {
			respond.Error(c, respond.ErrUnauthorized, respond.RequestID(c), c.Request.URL.Path)
			return
		}
		respond.SetPrincipal(c, p)
		c.Next()
	}
}

// RequireRole gates a route on the principal's database role (ADR-017: the role
// is read per request, so an admin disable is immediately effective). It must
// run after RequireAuth; a missing principal yields 401, a wrong role 403.
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := respond.PrincipalFrom(c)
		if !ok {
			respond.Error(c, respond.ErrUnauthorized, respond.RequestID(c), c.Request.URL.Path)
			return
		}
		if p.Role != role {
			respond.Error(c, respond.ErrForbidden, respond.RequestID(c), c.Request.URL.Path)
			return
		}
		c.Next()
	}
}

// RequireScope gates a route on an API-key scope (AUTH-05). A JWT session has
// full user rights (all scopes); an API key must carry the scope. It must run
// after RequireAuth.
func RequireScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := respond.PrincipalFrom(c)
		if !ok {
			respond.Error(c, respond.ErrUnauthorized, respond.RequestID(c), c.Request.URL.Path)
			return
		}
		if !p.HasScope(scope) {
			respond.Error(c, respond.ErrForbidden, respond.RequestID(c), c.Request.URL.Path)
			return
		}
		c.Next()
	}
}

// RateLimit enforces a per-IP token bucket (429 + Retry-After on exhaustion).
func RateLimit(limiter *ratelimit.KeyedLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !limiter.Allow(c.ClientIP()) {
			c.Header("Retry-After", "60")
			respond.Error(c, respond.ErrRateLimited, respond.RequestID(c), c.Request.URL.Path)
			return
		}
		c.Next()
	}
}

// CORS applies an explicit origin allowlist (no wildcards; security §3).
func CORS(allowOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowOrigins))
	for _, o := range allowOrigins {
		allowed[o] = struct{}{}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			if _, ok := allowed[origin]; ok {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Access-Control-Allow-Headers", "Authorization, X-API-Key, Idempotency-Key, X-Request-Id, Content-Type")
				c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				c.Header("Access-Control-Max-Age", "3600")
			}
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
