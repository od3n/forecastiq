// Package providerhttp is the provider-agnostic HTTP transport shared by
// forecast provider adapters. It centralizes the hardening that every adapter
// needs so provider packages stay thin and consistent:
//
//   - a bounded, redirect-capped HTTP client with a stable User-Agent;
//   - bounded response-body reads (defence against unbounded payloads);
//   - FC-08 retry with exponential backoff + jitter (retryable = network /
//     timeout / 5xx / 429; non-retryable = other 4xx);
//   - FC-13 classification into a structured ports.ProviderError;
//   - normalization of standard rate-limit headers (Retry-After, X-RateLimit-*).
//
// It performs no provider-specific parsing; adapters own schema validation and
// condition mapping. Base URLs are seeded provider configuration (not user
// input), so no SSRF surface exists here (security architecture §5).
package providerhttp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/forecastiq/forecastiq/internal/collection/ports"
	"github.com/forecastiq/forecastiq/internal/platform/ratelimit"
)

const (
	defaultUserAgent   = "ForecastIQ/1.0 (+https://forecastiq.local)"
	defaultMaxRespByte = 10 << 20 // 10 MB
	defaultMaxAttempts = 5        // FC-08: max 5 attempts
	defaultRetryBase   = time.Second
	defaultTimeout     = 10 * time.Second
	maxRedirects       = 5
)

// Config configures a Client. Zero values fall back to safe defaults.
type Config struct {
	HTTPClient       *http.Client
	UserAgent        string
	Limiter          *ratelimit.Limiter
	MaxResponseBytes int64
	MaxAttempts      int           // total attempts (FC-08 caps at 5)
	RetryBaseDelay   time.Duration // backoff base (1s); ±20% jitter
}

// Client is a hardened, reusable HTTP transport for provider adapters.
type Client struct {
	http         *http.Client
	limiter      *ratelimit.Limiter
	userAgent    string
	maxRespBytes int64
	maxAttempts  int
	retryBase    time.Duration
}

// New builds a Client, applying safe defaults and hardening the supplied (or a
// fresh) *http.Client: a capped redirect policy is always installed.
func New(cfg Config) *Client {
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: defaultTimeout}
	}
	// Cap redirects regardless of the caller's client (avoid redirect loops /
	// unexpected cross-host hops). Base URLs are trusted seeded config.
	hc.CheckRedirect = func(_ *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return fmt.Errorf("stopped after %d redirects", maxRedirects)
		}
		return nil
	}
	ua := cfg.UserAgent
	if ua == "" {
		ua = defaultUserAgent
	}
	maxBytes := cfg.MaxResponseBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxRespByte
	}
	attempts := cfg.MaxAttempts
	if attempts <= 0 {
		attempts = defaultMaxAttempts
	}
	base := cfg.RetryBaseDelay
	if base <= 0 {
		base = defaultRetryBase
	}
	return &Client{
		http: hc, limiter: cfg.Limiter, userAgent: ua,
		maxRespBytes: maxBytes, maxAttempts: attempts, retryBase: base,
	}
}

// Response is a successful (or terminally classified) fetch. Body/StatusCode/
// LatencyMS are best-effort populated even when a *ports.ProviderError is
// returned, so adapters can still persist the raw error payload (ADR-011).
type Response struct {
	Body       []byte
	StatusCode int
	LatencyMS  int
	RequestID  string
	RateLimit  *ports.RateLimit
}

// Get performs a bounded GET with FC-08 retry. It returns a non-nil *Response
// (best-effort) and, on failure, a classified *ports.ProviderError.
func (c *Client) Get(ctx context.Context, endpoint string, header http.Header) (*Response, *ports.ProviderError) {
	resp := &Response{}
	var last *ports.ProviderError
	for attempt := 0; attempt < c.maxAttempts; attempt++ {
		if attempt > 0 {
			if err := c.backoff(ctx, attempt); err != nil {
				return resp, ports.NewProviderError(ports.ErrTimeout, resp.StatusCode, false, err)
			}
		}
		if c.limiter != nil {
			if err := c.limiter.Wait(ctx); err != nil {
				return resp, ports.NewProviderError(ports.ErrTimeout, resp.StatusCode, false, err)
			}
		}
		body, code, rl, reqID, latency, err := c.doOnce(ctx, endpoint, header)
		resp.LatencyMS = latency
		if code > 0 {
			resp.StatusCode = code
		}
		if reqID != "" {
			resp.RequestID = reqID
		}
		if rl != nil {
			resp.RateLimit = rl
		}
		if len(body) > 0 {
			resp.Body = body
		}

		if err == nil && code >= 200 && code < 300 {
			return resp, nil // success
		}
		last = classify(code, rl, err)
		if !last.Retryable {
			return resp, last
		}
	}
	return resp, last
}

// doOnce performs a single request with a bounded body read.
func (c *Client) doOnce(ctx context.Context, endpoint string, header http.Header) (
	body []byte, status int, rl *ports.RateLimit, requestID string, latencyMS int, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, nil, "", 0, err
	}
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	req.Header.Set("User-Agent", c.userAgent)
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}

	start := time.Now()
	res, err := c.http.Do(req)
	latencyMS = int(time.Since(start).Milliseconds())
	if err != nil {
		return nil, 0, nil, "", latencyMS, err
	}
	defer func() { _ = res.Body.Close() }()

	rl = parseRateLimit(res.Header)
	requestID = res.Header.Get("X-Request-Id")
	limited := io.LimitReader(res.Body, c.maxRespBytes+1)
	body, err = io.ReadAll(limited)
	if err != nil {
		return nil, res.StatusCode, rl, requestID, latencyMS, err
	}
	if int64(len(body)) > c.maxRespBytes {
		return nil, res.StatusCode, rl, requestID, latencyMS,
			fmt.Errorf("response exceeds %d bytes", c.maxRespBytes)
	}
	return body, res.StatusCode, rl, requestID, latencyMS, nil
}

// classify maps a transport error / HTTP status onto a structured FC-13
// ProviderError with the correct retry disposition (FC-08).
func classify(status int, rl *ports.RateLimit, err error) *ports.ProviderError {
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
			return ports.NewProviderError(ports.ErrTimeout, status, true, err)
		}
		// Non-timeout transport failures with a 5xx status are provider-side;
		// otherwise the failure is our-side network reachability.
		if status >= 500 {
			return ports.NewProviderError(ports.ErrProvider5xx, status, true, err)
		}
		return ports.NewProviderError(ports.ErrNetworkLocal, status, true, err)
	}
	switch {
	case status == http.StatusTooManyRequests:
		pe := ports.NewProviderError(ports.ErrRateLimited, status, true, nil)
		pe.RateLimit = rl
		return pe
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return ports.NewProviderError(ports.ErrInvalidCredentials, status, false, nil)
	case status >= 500:
		return ports.NewProviderError(ports.ErrProvider5xx, status, true, nil)
	case status >= 400:
		// Other 4xx: request-contract breakage; not retryable. Classified as
		// network_local (our-side) per FC-13 system bucket.
		return ports.NewProviderError(ports.ErrNetworkLocal, status, false, nil)
	default:
		return ports.NewProviderError(ports.ErrNetworkLocal, status, false, nil)
	}
}

// backoff sleeps for the FC-08 delay of the given attempt (1,2,4,8,16s ±20%).
func (c *Client) backoff(ctx context.Context, attempt int) error {
	base := float64(c.retryBase) * float64(int(1)<<(attempt-1))
	jitter := 1.0 + (rand.Float64()*0.4 - 0.2) //nolint:gosec // jitter, not security
	delay := time.Duration(base * jitter)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}

// parseRateLimit normalizes standard rate-limit response headers. Returns nil
// when the provider signalled nothing.
func parseRateLimit(h http.Header) *ports.RateLimit {
	rl := &ports.RateLimit{Remaining: -1}
	found := false
	if v := h.Get("Retry-After"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil {
			d := time.Duration(secs) * time.Second
			rl.RetryAfter = &d
			found = true
		} else if t, err := http.ParseTime(v); err == nil {
			d := time.Until(t)
			if d < 0 {
				d = 0
			}
			rl.RetryAfter = &d
			found = true
		}
	}
	if v := h.Get("X-RateLimit-Limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			rl.Limit = n
			found = true
		}
	}
	if v := h.Get("X-RateLimit-Remaining"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			rl.Remaining = n
			found = true
		}
	}
	if v := h.Get("X-RateLimit-Reset"); v != "" {
		if epoch, err := strconv.ParseInt(v, 10, 64); err == nil {
			t := time.Unix(epoch, 0).UTC()
			rl.Reset = &t
			found = true
		}
	}
	if !found {
		return nil
	}
	return rl
}

func isTimeout(err error) bool {
	var netErr interface{ Timeout() bool }
	return errors.As(err, &netErr) && netErr.Timeout()
}
