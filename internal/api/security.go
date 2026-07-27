package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/forecastiq/forecastiq/internal/api/respond"
)

// SecurityHeaders adds defense-in-depth response headers at the application
// layer (security architecture §4). Under ADR-033 the origin serves plain HTTP
// on :80 behind Cloudflare (proxied DNS) — so for the API these app-level
// headers are the ONLY header-setting layer, not a fallback behind Caddy.
//
// Header ownership under ADR-033:
//   - API response headers (below): this middleware.
//   - HSTS + Always-Use-HTTPS: Cloudflare zone settings (terraform/cloudflare.tf).
//     HSTS is deliberately NOT emitted here — it must come from the TLS
//     terminator, and the origin is plain HTTP; setting it app-side would be
//     inert (Cloudflare, not the browser, talks to the origin) and risky on
//     localhost/dev.
//   - Dashboard (static export) CSP + headers: Cloudflare Pages `web/public/_headers`.
//
// Headers applied:
//   - X-Content-Type-Options: nosniff (MIME sniffing prevention)
//   - X-Frame-Options: DENY (clickjacking protection)
//   - Referrer-Policy: strict-origin-when-cross-origin
//   - X-XSS-Protection: 0 (disabled; modern CSP preferred)
//   - Permissions-Policy: restrictive (no device APIs)
//   - Content-Security-Policy: default-src 'none'; frame-ancestors 'none'
//     (a JSON API renders nothing; this hardens any error/redirect page)
//   - Cache-Control: no-store for non-GET (mutations never cached)
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-XSS-Protection", "0")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")

		// Mutations (non-GET/HEAD/OPTIONS) must never be cached.
		if c.Request.Method != "GET" && c.Request.Method != "HEAD" && c.Request.Method != "OPTIONS" {
			h.Set("Cache-Control", "no-store")
		}

		c.Next()
	}
}

// RequestBodyLimit enforces a maximum request body size (security architecture
// §4: 1 MB). A declared oversize (Content-Length) is rejected immediately with
// a 413 problem envelope; chunked/streamed bodies are bounded via
// http.MaxBytesReader, whose *http.MaxBytesError is mapped to 413 by
// respond.Classify at read time (it errors.As-matches inside binding errors).
// A body of exactly maxBytes is allowed (MaxBytesReader is inclusive).
func RequestBodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxBytes {
			respond.Error(c, respond.ErrPayloadTooLarge, respond.RequestID(c), c.Request.URL.Path)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

// IsBodyTooLarge reports whether err (from reading a request body) was caused
// by the MaxBytesReader limit. respond.Classify performs the same check, so
// handlers using respond.Error map oversize bodies to 413 automatically; this
// helper remains for callers that inspect the error directly.
func IsBodyTooLarge(err error) bool {
	var mbe *http.MaxBytesError
	return errors.As(err, &mbe)
}
