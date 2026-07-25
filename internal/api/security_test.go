package api_test

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/internal/api"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/ratelimit"
)

// TestSecurityHeaders_Present verifies the defense-in-depth response headers
// are applied to every response (threat model: clickjacking, MIME sniffing).
func TestSecurityHeaders_Present(t *testing.T) {
	handler := api.SecurityHeaders()
	w := performRequest(handler, "GET", "/test", nil)

	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
	assert.Equal(t, "0", w.Header().Get("X-XSS-Protection"))
	assert.Contains(t, w.Header().Get("Permissions-Policy"), "camera=()")
}

// TestSecurityHeaders_MutationCacheControl verifies that mutation responses
// get Cache-Control: no-store (threat: cached sensitive responses).
func TestSecurityHeaders_MutationCacheControl(t *testing.T) {
	handler := api.SecurityHeaders()

	// POST should get no-store
	w := performRequest(handler, "POST", "/test", nil)
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))

	// GET should NOT get no-store (caching is handler-controlled)
	w = performRequest(handler, "GET", "/test", nil)
	assert.Empty(t, w.Header().Get("Cache-Control"))
}

// TestRequestBodyLimit_Rejects413 verifies that oversized requests are rejected
// (threat model §8: DoS via large bodies).
func TestRequestBodyLimit_Rejects413(t *testing.T) {
	handler := api.RequestBodyLimit(100) // 100 bytes max

	// Body that exceeds limit via Content-Length header
	w := performRequestWithBody(handler, "POST", "/test", strings.Repeat("x", 200))
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

// TestRequestBodyLimit_AllowsSmall verifies that small requests pass through.
func TestRequestBodyLimit_AllowsSmall(t *testing.T) {
	handler := api.RequestBodyLimit(1024)
	w := performRequestWithBody(handler, "POST", "/test", `{"name":"test"}`)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRequestBodyLimit_AllowsExactLimit is the DRB-WP25 off-by-one regression
// test: a body of EXACTLY maxBytes must be readable to EOF without error
// (http.MaxBytesReader is inclusive of the limit).
func TestRequestBodyLimit_AllowsExactLimit(t *testing.T) {
	const limit = 100
	handler := api.RequestBodyLimit(limit)
	w := performRequestWithBody(handler, "POST", "/test", strings.Repeat("x", limit))
	assert.Equal(t, http.StatusOK, w.Code, "a body of exactly maxBytes must be accepted")
}

// TestRequestBodyLimit_StreamedOverflow verifies that a body exceeding the
// limit WITHOUT a declared Content-Length (chunked) fails the read with a
// MaxBytesError (mapped to 413 by callers via IsBodyTooLarge).
func TestRequestBodyLimit_StreamedOverflow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.Use(api.RequestBodyLimit(50))
	r.POST("/test", func(c *gin.Context) {
		_, err := io.ReadAll(c.Request.Body)
		if api.IsBodyTooLarge(err) {
			c.Status(http.StatusRequestEntityTooLarge)
			return
		}
		require.NoError(t, err)
		c.Status(200)
	})
	req := httptest.NewRequest("POST", "/test", strings.NewReader(strings.Repeat("y", 200)))
	req.ContentLength = -1 // simulate chunked (no declared length)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

// TestCORS_RejectsUnknownOrigin verifies that non-allowlisted origins get no
// CORS headers (threat model §15: insecure CORS).
func TestCORS_RejectsUnknownOrigin(t *testing.T) {
	handler := api.CORS([]string{"https://app.forecastiq.example"})
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	w := httptest.NewRecorder()

	router := setupRouter(handler)
	router.ServeHTTP(w, req)

	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"),
		"unknown origin must not receive CORS headers")
}

// TestCORS_AllowsConfiguredOrigin verifies allowlisted origins get CORS headers.
func TestCORS_AllowsConfiguredOrigin(t *testing.T) {
	origin := "https://app.forecastiq.example"
	handler := api.CORS([]string{origin})
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", origin)
	w := httptest.NewRecorder()

	router := setupRouter(handler)
	router.ServeHTTP(w, req)

	assert.Equal(t, origin, w.Header().Get("Access-Control-Allow-Origin"))
}

// TestRateLimit_Returns429 verifies that rate-limited requests get 429 with
// Retry-After header (threat model §8: DoS).
func TestRateLimit_Returns429(t *testing.T) {
	// Create a limiter with capacity 1, so the second request is limited.
	limiter := setupLimiter(1)
	handler := api.RateLimit(limiter)

	// First request passes
	w := performRequestWithIP(handler, "1.2.3.4")
	require.Equal(t, http.StatusOK, w.Code)

	// Second request is rate-limited
	w = performRequestWithIP(handler, "1.2.3.4")
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.NotEmpty(t, w.Header().Get("Retry-After"))
}

// TestRecovery_NoStackTrace verifies that panic recovery produces a sanitized
// error without stack traces or internal details (threat model §6, §11).
func TestRecovery_NoStackTrace(t *testing.T) {
	handler := api.Recovery(discardLogger())

	w := performPanicRequest(handler)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	body := w.Body.String()
	assert.NotContains(t, body, "goroutine")
	assert.NotContains(t, body, ".go:")
	assert.NotContains(t, body, "runtime/debug")
}

// --- helpers ---

func performRequest(mw gin.HandlerFunc, method, path string, body io.Reader) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.Use(mw)
	r.Handle(method, path, func(c *gin.Context) { c.Status(200) })
	req := httptest.NewRequest(method, path, body)
	r.ServeHTTP(w, req)
	return w
}

func performRequestWithBody(mw gin.HandlerFunc, method, path, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.Use(mw)
	r.Handle(method, path, func(c *gin.Context) {
		// Surface read errors: a MaxBytesReader breach must yield 413, and any
		// other unexpected read error must fail the request (not be discarded —
		// the discarded error masked the DRB-WP25 off-by-one).
		if _, err := io.ReadAll(c.Request.Body); err != nil {
			if api.IsBodyTooLarge(err) {
				c.Status(http.StatusRequestEntityTooLarge)
			} else {
				c.Status(http.StatusInternalServerError)
			}
			return
		}
		c.Status(200)
	})
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
	r.ServeHTTP(w, req)
	return w
}

func performRequestWithIP(mw gin.HandlerFunc, ip string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.Use(mw)
	r.GET("/test", func(c *gin.Context) { c.Status(200) })
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = ip + ":12345"
	r.ServeHTTP(w, req)
	return w
}

func performPanicRequest(mw gin.HandlerFunc) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.Use(mw)
	r.GET("/panic", func(c *gin.Context) { panic("test panic") })
	req := httptest.NewRequest("GET", "/panic", nil)
	r.ServeHTTP(w, req)
	return w
}

func setupRouter(mw gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(mw)
	r.GET("/test", func(c *gin.Context) { c.Status(200) })
	return r
}

func setupLimiter(capacity int) *ratelimit.KeyedLimiter {
	return ratelimit.NewKeyedLimiter(float64(capacity), float64(capacity)/60.0, clock.Real{})
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
