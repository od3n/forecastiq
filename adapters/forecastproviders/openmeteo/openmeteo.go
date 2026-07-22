// Package openmeteo implements the Open-Meteo forecast provider adapter behind
// the collection module's ForecastProviderAdapter port. Transport hardening,
// FC-08 retry, and FC-13 classification live in the shared providerhttp helper;
// this package owns only Open-Meteo's request shape, schema (openmeteo-v1),
// normalization, and WMO→canonical condition mapping.
package openmeteo

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/forecastiq/forecastiq/adapters/forecastproviders/providerhttp"
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

// Adapter implements ports.ForecastProviderAdapter (and ports.ReplayDecoder)
// for Open-Meteo.
type Adapter struct {
	transport *providerhttp.Client
	logger    *slog.Logger
}

// New builds an Open-Meteo adapter over the shared hardened transport.
func New(cfg Config) *Adapter {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	transport := providerhttp.New(providerhttp.Config{
		HTTPClient:       cfg.Client,
		Limiter:          cfg.Limiter,
		MaxResponseBytes: cfg.MaxResponseBytes,
		MaxAttempts:      cfg.MaxRetries,
		RetryBaseDelay:   cfg.RetryBaseDelay,
	})
	return &Adapter{transport: transport, logger: logger}
}

// Slug implements ports.ForecastProviderAdapter.
func (a *Adapter) Slug() string { return ProviderSlug }

// SchemaVersion implements ports.ForecastProviderAdapter.
func (a *Adapter) SchemaVersion() string { return SchemaVersion }

// AdapterVersion implements ports.ForecastProviderAdapter.
func (a *Adapter) AdapterVersion() string { return AdapterVersion }

// Capabilities implements ports.ForecastProviderAdapter. Open-Meteo needs no
// credential and supports deterministic replay from stored payloads.
func (a *Adapter) Capabilities() ports.Capabilities {
	return ports.Capabilities{
		MaxForecastHorizon: forecastDays * 24 * time.Hour,
		HourlyResolution:   true,
		RequiresCredential: false,
		SupportsReplay:     true,
	}
}

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
// normalize chain (workflow §2). Retry/rate-limit/classification are handled by
// the shared transport; this method maps a classified failure onto the result
// or, on success, decomposes the payload.
func (a *Adapter) FetchForecast(ctx context.Context, req ports.ForecastRequest) (*ports.ForecastResult, error) {
	result := &ports.ForecastResult{
		SchemaVersion:      SchemaVersion,
		AdapterVersion:     AdapterVersion,
		IssuedAt:           req.IssuedAt,
		UnmappedConditions: map[string]int{},
	}

	header := http.Header{}
	if req.Credential != "" {
		header.Set("Authorization", "Bearer "+req.Credential)
	}

	resp, ferr := a.transport.Get(ctx, a.buildURL(req), header)
	result.HTTPStatusCode = resp.StatusCode
	result.LatencyMS = resp.LatencyMS
	result.RawPayload = resp.Body
	result.ProviderRequestID = resp.RequestID
	result.RateLimit = resp.RateLimit
	if len(resp.Body) > 0 {
		result.Checksum = ports.Checksum(resp.Body)
	}

	if ferr != nil {
		result.Outcome = ferr.Outcome()
		result.ErrorCode = ferr.Code.String()
		result.Err = ferr
		if result.RateLimit == nil {
			result.RateLimit = ferr.RateLimit
		}
		return result, nil
	}

	// Parse + validate + decompose + normalize.
	a.decompose(resp.Body, req, result)
	return result, nil
}

// DecodeStored implements ports.ReplayDecoder: deterministically re-derive a
// result from a previously stored payload with no network call (domain §4.8).
// HTTP metadata (status/latency) is intentionally absent on replay.
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
