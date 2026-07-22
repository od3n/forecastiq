package providerhttp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/adapters/forecastproviders/providerhttp"
	"github.com/forecastiq/forecastiq/internal/collection/ports"
)

func newClient(attempts int) *providerhttp.Client {
	return providerhttp.New(providerhttp.Config{
		HTTPClient:     &http.Client{Timeout: 2 * time.Second},
		MaxAttempts:    attempts,
		RetryBaseDelay: time.Nanosecond, // keep retries instant in tests
	})
}

func TestGet_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.NotEmpty(t, r.Header.Get("User-Agent"), "hardened client must send User-Agent")
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	resp, ferr := newClient(1).Get(context.Background(), srv.URL, nil)
	require.Nil(t, ferr)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, `{"ok":true}`, string(resp.Body))
}

func TestGet_Classification(t *testing.T) {
	cases := []struct {
		name      string
		status    int
		wantCode  ports.ErrorCode
		retryable bool
	}{
		{"rate_limited", http.StatusTooManyRequests, ports.ErrRateLimited, true},
		{"unauthorized", http.StatusUnauthorized, ports.ErrInvalidCredentials, false},
		{"forbidden", http.StatusForbidden, ports.ErrInvalidCredentials, false},
		{"server_error", http.StatusInternalServerError, ports.ErrProvider5xx, true},
		{"bad_request", http.StatusBadRequest, ports.ErrNetworkLocal, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"error":"x"}`))
			}))
			defer srv.Close()

			resp, ferr := newClient(1).Get(context.Background(), srv.URL, nil)
			require.NotNil(t, ferr)
			assert.Equal(t, tc.wantCode, ferr.Code)
			assert.Equal(t, tc.retryable, ferr.Retryable)
			assert.Equal(t, tc.status, resp.StatusCode)
			// Best-effort raw body retained even on failure (ADR-011).
			assert.Equal(t, `{"error":"x"}`, string(resp.Body))
		})
	}
}

func TestGet_ErrorMessageRedaction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"secret":"leak-me"}`))
	}))
	defer srv.Close()

	_, ferr := newClient(1).Get(context.Background(), srv.URL, nil)
	require.NotNil(t, ferr)
	assert.NotContains(t, ferr.Error(), "leak-me", "provider error must not render the body")
	assert.Contains(t, ferr.Error(), "provider_5xx")
}

func TestGet_RetryThenSuccess(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	resp, ferr := newClient(5).Get(context.Background(), srv.URL, nil)
	require.Nil(t, ferr)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(3), calls.Load())
}

func TestGet_NonRetryableStopsImmediately(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, ferr := newClient(5).Get(context.Background(), srv.URL, nil)
	require.NotNil(t, ferr)
	assert.Equal(t, ports.ErrInvalidCredentials, ferr.Code)
	assert.Equal(t, int32(1), calls.Load(), "non-retryable failures must not retry")
}

func TestGet_RateLimitHeadersParsed(t *testing.T) {
	reset := time.Now().Add(time.Hour).Unix()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.Header().Set("X-RateLimit-Limit", "100")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	resp, ferr := newClient(1).Get(context.Background(), srv.URL, nil)
	require.NotNil(t, ferr)
	require.NotNil(t, ferr.RateLimit)
	assert.Equal(t, ports.ErrRateLimited, ferr.Code)
	require.NotNil(t, ferr.RateLimit.RetryAfter)
	assert.Equal(t, 30*time.Second, *ferr.RateLimit.RetryAfter)
	assert.Equal(t, 100, ferr.RateLimit.Limit)
	assert.Equal(t, 0, ferr.RateLimit.Remaining)
	require.NotNil(t, ferr.RateLimit.Reset)
	// Surfaced on the response too for the success/metadata path.
	assert.NotNil(t, resp.RateLimit)
}

func TestGet_BoundedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(make([]byte, 1024))
	}))
	defer srv.Close()

	client := providerhttp.New(providerhttp.Config{
		HTTPClient:       &http.Client{Timeout: 2 * time.Second},
		MaxAttempts:      1,
		MaxResponseBytes: 16, // smaller than the 1 KiB body
		RetryBaseDelay:   time.Nanosecond,
	})
	_, ferr := client.Get(context.Background(), srv.URL, nil)
	require.NotNil(t, ferr, "oversized response must fail closed")
}

func TestGet_RedirectCapped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.URL.String(), http.StatusFound) // self-redirect loop
	}))
	defer srv.Close()

	_, ferr := newClient(1).Get(context.Background(), srv.URL, nil)
	require.NotNil(t, ferr, "redirect loop must be capped and classified as failure")
}
