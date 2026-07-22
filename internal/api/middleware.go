// Package api is the HTTP layer: routing, middleware (request id, logging,
// recovery, metrics, auth, rate limit, CORS), and thin handlers that call
// module use cases and assemble envelopes (module architecture §3.5). Handlers
// contain no business logic.
package api

import (
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/api/respond"
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

// authenticate resolves the caller from the dev-token seam. Until WP-03/19
// lands Supabase JWKS auth, a configured FIQ_DEV_ADMIN_TOKEN grants an admin
// principal; with no token configured, protected routes are rejected.
func authenticate(c *gin.Context, devToken string) (respond.Principal, bool) {
	if devToken == "" {
		return respond.Principal{}, false
	}
	token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	if token == "" {
		token = c.GetHeader("X-API-Key")
	}
	if token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(devToken)) == 1 {
		return respond.Principal{Role: "admin", Name: "dev-admin"}, true
	}
	return respond.Principal{}, false
}

// RequireAuth gates user+ routes.
func RequireAuth(devToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := authenticate(c, devToken)
		if !ok {
			respond.Error(c, respond.ErrUnauthorized, respond.RequestID(c), c.Request.URL.Path)
			return
		}
		respond.SetPrincipal(c, p)
		c.Next()
	}
}

// RequireAdmin gates admin routes.
func RequireAdmin(devToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := authenticate(c, devToken)
		if !ok {
			respond.Error(c, respond.ErrUnauthorized, respond.RequestID(c), c.Request.URL.Path)
			return
		}
		if !p.IsAdmin() {
			respond.Error(c, respond.ErrForbidden, respond.RequestID(c), c.Request.URL.Path)
			return
		}
		respond.SetPrincipal(c, p)
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
