// Package openweather implements the OpenWeather One Call 3.0 forecast provider
// adapter behind the collection module's ForecastProviderAdapter port (WP-07).
// Transport hardening, FC-08 retry, and FC-13 classification live in the shared
// providerhttp helper; this package owns only OpenWeather's request shape,
// schema (openweather-v1), normalization, WMO-equivalent condition mapping, and
// the daily rate-budget guard (429 → pause) that keeps the adapter inside the
// provider's free-tier daily call budget.
package openweather

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/forecastiq/forecastiq/adapters/forecastproviders/providerhttp"
	"github.com/forecastiq/forecastiq/internal/collection/ports"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/ratelimit"
)

// Adapter identity (recorded on every collection row; workflow §9).
const (
	ProviderSlug   = "openweather"
	SchemaVersion  = "openweather-v1"
	AdapterVersion = "1.0.0"

	// One Call 3.0 hourly forecast: 48 periods. We exclude the sections we do
	// not decompose to keep the payload small and the contract narrow.
	forecastPath   = "/data/3.0/onecall"
	excludeParts   = "current,minutely,daily,alerts"
	forecastPeriod = 48 * time.Hour
	// DefaultDailyBudget is the One Call 3.0 free-tier daily call allowance.
	// The budget guard pauses collection once this is spent for the UTC day.
	DefaultDailyBudget = 1000
)

// Config configures an Adapter. Zero values fall back to safe defaults.
type Config struct {
	Client           *http.Client
	Limiter          *ratelimit.Limiter
	Logger           *slog.Logger
	Clock            clock.Clock
	MaxResponseBytes int64
	MaxRetries       int           // FC-08: max 5 attempts
	RetryBaseDelay   time.Duration // backoff base (1s); ±20% jitter
	// DailyBudget caps upstream calls per UTC day. Zero disables the guard;
	// negative is treated as zero (disabled).
	DailyBudget int
}

// Adapter implements ports.ForecastProviderAdapter (and ports.ReplayDecoder)
// for OpenWeather One Call 3.0.
type Adapter struct {
	transport *providerhttp.Client
	logger    *slog.Logger
	clock     clock.Clock
	budget    *dailyBudget // nil when no daily budget is configured
}

// New builds an OpenWeather adapter over the shared hardened transport.
func New(cfg Config) *Adapter {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	clk := cfg.Clock
	if clk == nil {
		clk = clock.Real{}
	}
	transport := providerhttp.New(providerhttp.Config{
		HTTPClient:       cfg.Client,
		Limiter:          cfg.Limiter,
		MaxResponseBytes: cfg.MaxResponseBytes,
		MaxAttempts:      cfg.MaxRetries,
		RetryBaseDelay:   cfg.RetryBaseDelay,
		// A 429 from OpenWeather signals the daily allowance is spent, not a
		// transient burst: retrying only burns quota and delays the pause. The
		// budget guard handles it with a pause, so opt 429 out of transport
		// retry (5xx/timeout still retry per FC-08).
		RetryableOverride: func(status int, def bool) bool {
			if status == http.StatusTooManyRequests {
				return false
			}
			return def
		},
	})
	a := &Adapter{transport: transport, logger: logger, clock: clk}
	if cfg.DailyBudget > 0 {
		a.budget = newDailyBudget(cfg.DailyBudget, clk)
	}
	return a
}

// Slug implements ports.ForecastProviderAdapter.
func (a *Adapter) Slug() string { return ProviderSlug }

// SchemaVersion implements ports.ForecastProviderAdapter.
func (a *Adapter) SchemaVersion() string { return SchemaVersion }

// AdapterVersion implements ports.ForecastProviderAdapter.
func (a *Adapter) AdapterVersion() string { return AdapterVersion }

// Capabilities implements ports.ForecastProviderAdapter. OpenWeather requires
// an API key and supports deterministic replay from stored payloads.
func (a *Adapter) Capabilities() ports.Capabilities {
	return ports.Capabilities{
		MaxForecastHorizon: forecastPeriod,
		HourlyResolution:   true,
		RequiresCredential: true,
		SupportsReplay:     true,
	}
}

// FetchForecast implements the provider call → validate → decompose →
// normalize chain (workflow §2). The daily budget guard runs first: when the
// budget is spent or a prior 429 engaged a pause, the call is refused
// pre-emptively with a rate_limited outcome and no upstream request is made.
// Retry/rate-limit/classification are otherwise handled by the shared
// transport; a classified failure is mapped onto the result and a 429 engages
// the pause so subsequent slots back off until the window resets.
func (a *Adapter) FetchForecast(ctx context.Context, req ports.ForecastRequest) (*ports.ForecastResult, error) {
	result := &ports.ForecastResult{
		SchemaVersion:      SchemaVersion,
		AdapterVersion:     AdapterVersion,
		IssuedAt:           req.IssuedAt,
		UnmappedConditions: map[string]int{},
	}

	now := a.clock.Now()
	if a.budget != nil {
		if ok, retryAfter := a.budget.reserve(now); !ok {
			// Budget spent or paused after a prior 429: refuse without calling
			// upstream (workflow §4 outbound budget; ADR-020).
			ra := retryAfter
			result.Outcome = ports.OutcomeRateLimited
			result.ErrorCode = ports.ErrRateLimited.String()
			result.RateLimit = &ports.RateLimit{Remaining: 0, RetryAfter: &ra}
			result.Err = ports.NewProviderError(ports.ErrRateLimited, 0, true, nil)
			return result, nil
		}
	}

	header := http.Header{}
	resp, ferr := a.transport.Get(ctx, a.buildURL(req), header)
	result.HTTPStatusCode = resp.StatusCode
	result.LatencyMS = resp.LatencyMS
	result.RawPayload = resp.Body
	result.ProviderRequestID = resp.RequestID
	result.RateLimit = resp.RateLimit
	if len(resp.Body) > 0 {
		result.Checksum = ports.Checksum(resp.Body)
	}
	// Account any extra upstream requests the transport made (FC-08 retries)
	// so the daily budget reflects real provider calls, not collection attempts.
	if a.budget != nil && resp.Attempts > 1 {
		a.budget.consume(resp.Attempts-1, a.clock.Now())
	}

	if ferr != nil {
		result.Outcome = ferr.Outcome()
		result.ErrorCode = ferr.Code.String()
		result.Err = ferr
		if result.RateLimit == nil {
			result.RateLimit = ferr.RateLimit
		}
		if ferr.Code == ports.ErrRateLimited && a.budget != nil {
			// Engage the pause: honor Retry-After when present, otherwise pause
			// until the next UTC day (429 → pause). Read a fresh clock so the
			// window is anchored to now, not the pre-call timestamp.
			var pauseFor time.Duration
			if ferr.RateLimit != nil && ferr.RateLimit.RetryAfter != nil {
				pauseFor = *ferr.RateLimit.RetryAfter
			}
			a.budget.pause(a.clock.Now(), pauseFor)
		}
		return result, nil
	}

	// Parse + validate + decompose + normalize.
	a.decompose(resp.Body, req, result)
	return result, nil
}

// DecodeStored implements ports.ReplayDecoder: deterministically re-derive a
// result from a previously stored payload with no network call (domain §4.8).
// HTTP metadata (status/latency) is intentionally absent on replay, and the
// budget guard is not consulted (no upstream call is made).
func (a *Adapter) DecodeStored(_ context.Context, req ports.ForecastRequest, raw []byte) (*ports.ForecastResult, error) {
	result := &ports.ForecastResult{
		SchemaVersion:      SchemaVersion,
		AdapterVersion:     AdapterVersion,
		IssuedAt:           req.IssuedAt,
		UnmappedConditions: map[string]int{},
		RawPayload:         raw,
	}
	if len(raw) > 0 {
		result.Checksum = ports.Checksum(raw)
	}
	a.decompose(raw, req, result)
	return result, nil
}

// buildURL assembles the One Call 3.0 request. The API key is passed as the
// appid query parameter (never logged; providerhttp does not log URLs), and
// units=metric pins °C / m/s at the source (domain §5).
func (a *Adapter) buildURL(req ports.ForecastRequest) string {
	base := req.BaseURL
	if base == "" {
		base = "https://api.openweathermap.org"
	}
	u := base + forecastPath
	q := url.Values{}
	q.Set("lat", strconv.FormatFloat(req.Latitude, 'f', 6, 64))
	q.Set("lon", strconv.FormatFloat(req.Longitude, 'f', 6, 64))
	q.Set("exclude", excludeParts)
	q.Set("units", "metric") // BR-PROV-01: canonical units at the source
	if req.Credential != "" {
		q.Set("appid", req.Credential)
	}
	return u + "?" + q.Encode()
}
