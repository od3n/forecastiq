// Package openmeteo (observation source) implements the Open-Meteo Historical
// observation adapter behind the collection module's ObservationSourceAdapter
// port (WP-09). Transport hardening, FC-08 retry, and FC-13 classification live
// in the shared providerhttp helper; this package owns only the historical
// request shape, schema (openmeteo-historical-v1), UTC normalization,
// provenance typing (reanalysis default, ADR-003 / decision A-4), OC-04
// range→suspect flagging, and WMO→canonical condition mapping.
//
// Observations carry no raw-payload storage or checksum (ADR-025): the source
// is the re-queryable truth reference. Correction detection against previously
// stored rows is the collection pipeline's concern (WP-10), using
// domain.Observation.DiffersFrom.
package openmeteo

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/forecastiq/forecastiq/adapters/forecastproviders/providerhttp"
	"github.com/forecastiq/forecastiq/internal/collection/domain"
	"github.com/forecastiq/forecastiq/internal/collection/ports"
	"github.com/forecastiq/forecastiq/internal/platform/ratelimit"
)

// Adapter identity (recorded for lineage; workflow §9).
const (
	Source         = "openmeteo_historical"
	SchemaVersion  = "openmeteo-historical-v1"
	AdapterVersion = "1.0.0"

	forecastPath = "/v1/forecast"
	// Hourly variables requested for observations (measured quantities only —
	// no probability/feels-like, which are forecast-only concepts).
	hourlyParams = "temperature_2m,relative_humidity_2m,wind_speed_10m," +
		"wind_direction_10m,surface_pressure,precipitation,weather_code"
	// hourTimeLayout is Open-Meteo's start_hour/end_hour and hourly.time format
	// when timezone=UTC ("2026-07-22T10:00").
	hourTimeLayout = "2006-01-02T15:04"
)

// Config configures an Adapter. Zero values fall back to safe defaults.
type Config struct {
	Client           *http.Client
	Limiter          *ratelimit.Limiter
	Logger           *slog.Logger
	MaxResponseBytes int64
	MaxRetries       int
	RetryBaseDelay   time.Duration
	// DefaultObservationType is the provenance assigned to every row (Open-Meteo
	// Historical does not expose per-variable provenance). Zero value falls back
	// to reanalysis (ADR-003 documented binding assumption).
	DefaultObservationType domain.ObservationType
}

// Adapter implements ports.ObservationSourceAdapter for Open-Meteo Historical.
type Adapter struct {
	transport   *providerhttp.Client
	logger      *slog.Logger
	defaultType domain.ObservationType
}

// New builds an Open-Meteo Historical observation adapter over the shared
// hardened transport.
func New(cfg Config) *Adapter {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	defType := cfg.DefaultObservationType
	if defType == "" {
		defType = domain.ObservationReanalysis
	}
	transport := providerhttp.New(providerhttp.Config{
		HTTPClient:       cfg.Client,
		Limiter:          cfg.Limiter,
		MaxResponseBytes: cfg.MaxResponseBytes,
		MaxAttempts:      cfg.MaxRetries,
		RetryBaseDelay:   cfg.RetryBaseDelay,
	})
	return &Adapter{transport: transport, logger: logger, defaultType: defType}
}

// Source implements ports.ObservationSourceAdapter.
func (a *Adapter) Source() string { return Source }

// SchemaVersion implements ports.ObservationSourceAdapter.
func (a *Adapter) SchemaVersion() string { return SchemaVersion }

// AdapterVersion implements ports.ObservationSourceAdapter.
func (a *Adapter) AdapterVersion() string { return AdapterVersion }

// FetchObservations performs the source call → validate → decompose →
// normalize chain (workflow §2). Retry/rate-limit/classification are handled by
// the shared transport; this method maps a classified failure onto the result
// or, on success, decomposes the payload into observation rows.
func (a *Adapter) FetchObservations(ctx context.Context, req ports.ObservationRequest) (*ports.ObservationResult, error) {
	result := &ports.ObservationResult{
		Source:         Source,
		SchemaVersion:  SchemaVersion,
		AdapterVersion: AdapterVersion,
	}

	resp, ferr := a.transport.Get(ctx, a.buildURL(req), http.Header{})
	result.HTTPStatusCode = resp.StatusCode
	result.LatencyMS = resp.LatencyMS
	result.RateLimit = resp.RateLimit

	if ferr != nil {
		result.Outcome = ferr.Outcome()
		result.ErrorCode = ferr.Code.String()
		result.Err = ferr
		if result.RateLimit == nil {
			result.RateLimit = ferr.RateLimit
		}
		return result, nil
	}

	a.decompose(resp.Body, req, result)
	return result, nil
}
