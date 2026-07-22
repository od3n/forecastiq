package openmeteo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/forecastiq/forecastiq/internal/collection/ports"
	"github.com/forecastiq/forecastiq/internal/platform/ratelimit"
)

// Adapter identity (recorded on every collection row; workflow §9).
const (
	ProviderSlug   = "open-meteo"
	SchemaVersion  = "openmeteo-v1"
	AdapterVersion = "1.0.0"

	forecastPath = "/v1/forecast"
	// Hourly variables requested; precipitation_probability is returned as a
	// 0–100 percentage and normalized to [0,1] (domain §5).
	hourlyParams = "temperature_2m,apparent_temperature,precipitation_probability," +
		"precipitation,relative_humidity_2m,wind_speed_10m,wind_direction_10m," +
		"surface_pressure,cloud_cover,weather_code"
	forecastDays = 7
)

// Config configures an Adapter. Zero values fall back to safe defaults.
type Config struct {
	Client           *http.Client
	Limiter          *ratelimit.Limiter
	Logger           *slog.Logger
	MaxResponseBytes int64
	MaxRetries       int           // FC-08: max 5 attempts
	RetryBaseDelay   time.Duration // backoff base (1s); ±20% jitter
}

// Adapter implements ports.ForecastProviderAdapter for Open-Meteo.
type Adapter struct {
	client       *http.Client
	limiter      *ratelimit.Limiter
	logger       *slog.Logger
	maxRespBytes int64
	maxRetries   int
	retryBase    time.Duration
}

// New builds an Open-Meteo adapter.
func New(cfg Config) *Adapter {
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: 10 * time.Second}
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.MaxResponseBytes <= 0 {
		cfg.MaxResponseBytes = 10 << 20 // 10 MB
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 5
	}
	if cfg.RetryBaseDelay <= 0 {
		cfg.RetryBaseDelay = time.Second
	}
	return &Adapter{
		client: cfg.Client, limiter: cfg.Limiter, logger: cfg.Logger,
		maxRespBytes: cfg.MaxResponseBytes, maxRetries: cfg.MaxRetries, retryBase: cfg.RetryBaseDelay,
	}
}

// Slug implements ports.ForecastProviderAdapter.
func (a *Adapter) Slug() string { return ProviderSlug }

// SchemaVersion implements ports.ForecastProviderAdapter.
func (a *Adapter) SchemaVersion() string { return SchemaVersion }

// AdapterVersion implements ports.ForecastProviderAdapter.
func (a *Adapter) AdapterVersion() string { return AdapterVersion }

// ── Response schema (openmeteo-v1) ────────────────────────────────────

type forecastResponse struct {
	Latitude  float64     `json:"latitude"`
	Longitude float64     `json:"longitude"`
	Timezone  string      `json:"timezone"`
	Hourly    *hourlyData `json:"hourly"`
}

type hourlyData struct {
	Time                     []string   `json:"time"`
	Temperature2m            []*float64 `json:"temperature_2m"`
	ApparentTemperature      []*float64 `json:"apparent_temperature"`
	PrecipitationProbability []*float64 `json:"precipitation_probability"`
	Precipitation            []*float64 `json:"precipitation"`
	RelativeHumidity2m       []*float64 `json:"relative_humidity_2m"`
	WindSpeed10m             []*float64 `json:"wind_speed_10m"`
	WindDirection10m         []*float64 `json:"wind_direction_10m"`
	SurfacePressure          []*float64 `json:"surface_pressure"`
	CloudCover               []*float64 `json:"cloud_cover"`
	WeatherCode              []*int     `json:"weather_code"`
}

// FetchForecast implements the provider call → validate → decompose →
// normalize chain (workflow §2). Retries retryable failures with backoff.
func (a *Adapter) FetchForecast(ctx context.Context, req ports.ForecastRequest) (*ports.ForecastResult, error) {
	result := &ports.ForecastResult{
		SchemaVersion:      SchemaVersion,
		AdapterVersion:     AdapterVersion,
		IssuedAt:           req.IssuedAt,
		UnmappedConditions: map[string]int{},
	}

	raw, status, latencyMS, err := a.fetchWithRetry(ctx, req)
	result.HTTPStatusCode = status
	result.LatencyMS = latencyMS
	result.RawPayload = raw
	if len(raw) > 0 {
		result.Checksum = ports.Checksum(raw)
	}

	// Transport / HTTP-level outcomes (no parseable success body).
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
			result.Outcome = ports.OutcomeTimeout
			result.ErrorCode = "timeout"
		} else {
			result.Outcome = ports.OutcomeFailed
			result.ErrorCode = "network"
		}
		result.Err = err
		return result, nil
	}
	switch {
	case status == http.StatusTooManyRequests:
		result.Outcome = ports.OutcomeRateLimited
		result.ErrorCode = "rate_limited"
		return result, nil
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		result.Outcome = ports.OutcomeAuthFailed
		result.ErrorCode = "invalid_credentials"
		return result, nil
	case status >= 400:
		result.Outcome = ports.OutcomeFailed
		result.ErrorCode = "http_" + strconv.Itoa(status)
		return result, nil
	}

	// Parse + validate + decompose + normalize.
	a.decompose(raw, req, result)
	return result, nil
}

// fetchWithRetry performs the HTTP GET with rate-limit awareness and FC-08
// backoff (1,2,4,8,16 s ±20% jitter; retryable: network, timeout, 5xx, 429).
func (a *Adapter) fetchWithRetry(ctx context.Context, req ports.ForecastRequest) (raw []byte, status, latencyMS int, err error) {
	endpoint := a.buildURL(req)
	var lastErr error
	for attempt := 0; attempt < a.maxRetries; attempt++ {
		if attempt > 0 {
			if werr := a.backoff(ctx, attempt); werr != nil {
				return nil, 0, 0, werr
			}
		}
		if a.limiter != nil {
			if werr := a.limiter.Wait(ctx); werr != nil {
				return nil, 0, 0, werr
			}
		}
		body, code, latency, rerr := a.doOnce(ctx, endpoint, req.Credential)
		if rerr == nil && code >= 200 && code < 300 {
			return body, code, latency, nil
		}
		lastErr = rerr
		status, latencyMS = code, latency
		if body != nil {
			raw = body
		}
		if !retryable(code, rerr) {
			return raw, code, latency, rerr
		}
	}
	if lastErr == nil && status >= 500 {
		lastErr = fmt.Errorf("provider returned %d after %d attempts", status, a.maxRetries)
	}
	return raw, status, latencyMS, lastErr
}

func (a *Adapter) doOnce(ctx context.Context, endpoint, credential string) (body []byte, status, latencyMS int, err error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, 0, err
	}
	httpReq.Header.Set("Accept", "application/json")
	if credential != "" {
		httpReq.Header.Set("Authorization", "Bearer "+credential)
	}
	start := time.Now()
	resp, err := a.client.Do(httpReq)
	latencyMS = int(time.Since(start).Milliseconds())
	if err != nil {
		return nil, 0, latencyMS, err
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, a.maxRespBytes+1)
	body, err = io.ReadAll(limited)
	if err != nil {
		return nil, resp.StatusCode, latencyMS, err
	}
	if int64(len(body)) > a.maxRespBytes {
		return nil, resp.StatusCode, latencyMS, fmt.Errorf("response exceeds %d bytes", a.maxRespBytes)
	}
	return body, resp.StatusCode, latencyMS, nil
}

func (a *Adapter) buildURL(req ports.ForecastRequest) string {
	base := req.BaseURL
	if base == "" {
		base = "https://api.open-meteo.com"
	}
	u := base + forecastPath
	q := url.Values{}
	q.Set("latitude", strconv.FormatFloat(req.Latitude, 'f', 6, 64))
	q.Set("longitude", strconv.FormatFloat(req.Longitude, 'f', 6, 64))
	q.Set("hourly", hourlyParams)
	q.Set("forecast_days", strconv.Itoa(forecastDays))
	q.Set("timezone", "UTC") // BR-PROV-01: normalize to UTC at the source
	return u + "?" + q.Encode()
}

func (a *Adapter) backoff(ctx context.Context, attempt int) error {
	base := float64(a.retryBase) * float64(int(1)<<(attempt-1)) // 1,2,4,8,16...
	jitter := 1.0 + (rand.Float64()*0.4 - 0.2)                  // ±20%
	delay := time.Duration(base * jitter)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}

func retryable(status int, err error) bool {
	if err != nil {
		return true // network / timeout
	}
	return status == http.StatusTooManyRequests || status >= 500
}

func isTimeout(err error) bool {
	var netErr interface{ Timeout() bool }
	return errors.As(err, &netErr) && netErr.Timeout()
}
