package api

import "github.com/gin-gonic/gin"

// SecurityHeaders adds defense-in-depth response headers at the application
// layer (security architecture §4). Caddy is the primary enforcement point
// (deploy/caddy/Caddyfile), but these headers protect against misconfigured
// reverse proxies or direct-to-app access during development.
//
// Headers applied:
//   - X-Content-Type-Options: nosniff (MIME sniffing prevention)
//   - X-Frame-Options: DENY (clickjacking protection)
//   - Referrer-Policy: strict-origin-when-cross-origin
//   - X-XSS-Protection: 0 (disabled; modern CSP preferred)
//   - Permissions-Policy: restrictive (no device APIs)
//   - Cache-Control: no-store for non-GET (mutations never cached)
//
// HSTS is NOT set here — it must only come from the TLS terminator (Caddy)
// to avoid issues with localhost/dev environments.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-XSS-Protection", "0")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

		// Mutations (non-GET/HEAD/OPTIONS) must never be cached.
		if c.Request.Method != "GET" && c.Request.Method != "HEAD" && c.Request.Method != "OPTIONS" {
			h.Set("Cache-Control", "no-store")
		}

		c.Next()
	}
}

// RequestBodyLimit enforces a maximum request body size (security architecture
// §4: 1 MB). Requests exceeding the limit receive 413 Payload Too Large.
func RequestBodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxBytes {
			c.AbortWithStatus(413)
			return
		}
		c.Request.Body = &limitedReader{c.Request.Body, maxBytes}
		c.Next()
	}
}

// limitedReader wraps a reader to enforce byte limits; returns an error after
// the limit is exceeded so partially-streamed large bodies are rejected.
type limitedReader struct {
	inner     interface{ Read([]byte) (int, error) }
	remaining int64
}

func (lr *limitedReader) Read(p []byte) (int, error) {
	if lr.remaining <= 0 {
		return 0, &bodyTooLargeError{}
	}
	if int64(len(p)) > lr.remaining {
		p = p[:lr.remaining]
	}
	n, err := lr.inner.Read(p)
	lr.remaining -= int64(n)
	return n, err
}

func (lr *limitedReader) Close() error {
	if closer, ok := lr.inner.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

type bodyTooLargeError struct{}

func (e *bodyTooLargeError) Error() string { return "request body too large" }
