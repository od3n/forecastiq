// Package handlers contains the thin HTTP handlers for the first-slice
// endpoints. Handlers parse requests, call module use cases, and assemble
// envelopes — no business logic (module architecture §3.5).
package handlers

import (
	"context"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/admin"
	"github.com/forecastiq/forecastiq/internal/analysis"
	analysisdomain "github.com/forecastiq/forecastiq/internal/analysis/domain"
	analysisports "github.com/forecastiq/forecastiq/internal/analysis/ports"
	"github.com/forecastiq/forecastiq/internal/api/respond"
	"github.com/forecastiq/forecastiq/internal/audit"
	"github.com/forecastiq/forecastiq/internal/catalog"
	"github.com/forecastiq/forecastiq/internal/collection"
	collectiondomain "github.com/forecastiq/forecastiq/internal/collection/domain"
	"github.com/forecastiq/forecastiq/internal/identity"
	"github.com/forecastiq/forecastiq/internal/platform/health"
)

// RankingReader is the analysis read surface the public dashboard endpoints
// consume (WP-15). Implemented by *analysis.ReadService.
type RankingReader interface {
	Rankings(ctx context.Context, q analysis.RankingsQuery) (*analysis.RankingsResult, error)
	Methodology() analysisdomain.MethodologyDoc
	LocationSummary(ctx context.Context, locationID uuid.UUID, horizonMinutes int) (*analysis.LocationSummary, error)
	ProviderSummary(ctx context.Context, providerID uuid.UUID) (*analysis.ProviderSummary, error)
	Trends(ctx context.Context, f analysisports.TrendFilter, loc *time.Location) (*analysis.TrendsResult, error)
	ForecastComparison(ctx context.Context, q analysis.ComparisonQuery) (*analysis.ComparisonResult, error)
}

// HealthReader assembles the S-10 admin collection-health view (WP-18).
// Implemented by *admin.HealthService.
type HealthReader interface {
	Assemble(ctx context.Context) (*admin.Health, error)
}

// Recomputer runs an on-demand analysis recompute (S-13 admin; WP-18).
// Implemented by *admin.RecomputeService.
type Recomputer interface {
	Recompute(ctx context.Context, actor admin.RecomputeActor) (int, error)
}

// UserProfile serves + updates the current user's profile (WP-19 self-service).
// Implemented by *identity.UserService.
type UserProfile interface {
	GetMe(ctx context.Context, userID uuid.UUID) (*identity.User, error)
	UpdateMe(ctx context.Context, userID uuid.UUID, in identity.UpdateProfileInput) (*identity.User, error)
}

// APIKeyManager manages a user's personal API keys (WP-19 self-service).
// Implemented by *identity.APIKeyService.
type APIKeyManager interface {
	CreateKey(ctx context.Context, p identity.Principal, in identity.CreateKeyInput) (*identity.CreatedKey, error)
	ListKeys(ctx context.Context, userID uuid.UUID) ([]*identity.APIKey, error)
	RevokeKey(ctx context.Context, userID, keyID uuid.UUID, actor identity.Actor) error
}

// Handlers bundles the module services the slice endpoints need.
type Handlers struct {
	Locations         catalog.LocationManager
	Providers         catalog.ProviderCatalog
	Configs           catalog.ConfigurationManager
	ProviderAdmin     catalog.ProviderAdmin
	ConfigAdmin       catalog.ConfigurationAdmin
	Collector         collection.ForecastCollector
	Replayer          collection.ForecastReplayer
	Reader            collection.ForecastReader
	Analysis          RankingReader
	AdminHealthReader HealthReader
	Audit             audit.Reader
	Recompute         Recomputer
	Users             UserProfile
	Keys              APIKeyManager
	Health            *health.Checker
	Logger            *slog.Logger
}

// ── DTOs ──────────────────────────────────────────────────────────────

// LocationDTO is the public location representation.
type LocationDTO struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	CountryCode string    `json:"country_code"`
	Latitude    float64   `json:"latitude"`
	Longitude   float64   `json:"longitude"`
	Timezone    string    `json:"timezone"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

func locationDTO(l *catalog.Location) LocationDTO {
	return LocationDTO{
		ID: l.ID.String(), Name: l.Name, CountryCode: l.CountryCode,
		Latitude: l.Latitude, Longitude: l.Longitude, Timezone: l.Timezone,
		Status: string(l.Status), CreatedAt: l.CreatedAt,
	}
}

// AttributionDTO credits a provider.
type AttributionDTO struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

// ProviderDTO is the public provider representation.
type ProviderDTO struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Slug            string         `json:"slug"`
	Status          string         `json:"status"`
	Attribution     AttributionDTO `json:"attribution"`
	AdapterVersion  string         `json:"adapter_version,omitempty"`
	CollectingSince *time.Time     `json:"collecting_since,omitempty"`
}

func providerDTO(p *catalog.Provider) ProviderDTO {
	return ProviderDTO{
		ID: p.ID.String(), Name: p.Name, Slug: p.Slug, Status: string(p.Status),
		Attribution: AttributionDTO{Text: p.AttributionText, URL: p.AttributionURL},
	}
}

// CollectionDTO is the admin collection-lineage representation.
type CollectionDTO struct {
	ID                    string     `json:"id"`
	ProviderID            string     `json:"provider_id"`
	LocationID            string     `json:"location_id"`
	Status                string     `json:"status"`
	RequestedAt           time.Time  `json:"requested_at"`
	CompletedAt           *time.Time `json:"completed_at,omitempty"`
	RecordsReceived       int        `json:"records_received"`
	SnapshotsStored       int        `json:"snapshots_stored"`
	SnapshotsDeduplicated int        `json:"snapshots_deduplicated"`
	SnapshotsInvalid      int        `json:"snapshots_invalid"`
	SchemaVersion         string     `json:"schema_version,omitempty"`
	AdapterVersion        string     `json:"adapter_version,omitempty"`
	ErrorCode             string     `json:"error_code,omitempty"`
	ErrorMessage          string     `json:"error_message,omitempty"`
	RawPayloadObjectKey   string     `json:"raw_payload_object_key,omitempty"`
	RawPayloadChecksum    string     `json:"raw_payload_checksum,omitempty"`
	ResponseStatusCode    int        `json:"response_status_code,omitempty"`
	ResponseLatencyMS     int        `json:"response_latency_ms,omitempty"`
}

func collectionDTO(c *collectiondomain.ForecastCollection) CollectionDTO {
	return CollectionDTO{
		ID: c.ID.String(), ProviderID: c.ProviderID.String(), LocationID: c.LocationID.String(),
		Status: string(c.Status), RequestedAt: c.RequestedAt, CompletedAt: c.CompletedAt,
		RecordsReceived: c.RecordsReceived, SnapshotsStored: c.SnapshotsStored,
		SnapshotsDeduplicated: c.SnapshotsDeduplicated, SnapshotsInvalid: c.SnapshotsInvalid,
		SchemaVersion: c.SchemaVersion, AdapterVersion: c.AdapterVersion,
		ErrorCode: c.ErrorCode, ErrorMessage: c.ErrorMessage,
		RawPayloadObjectKey: c.RawPayloadObjectKey, RawPayloadChecksum: c.RawPayloadChecksum,
		ResponseStatusCode: c.ResponseStatusCode, ResponseLatencyMS: c.ResponseLatencyMS,
	}
}

// SnapshotDTO is the normalized forecast snapshot representation.
type SnapshotDTO struct {
	ID                       string    `json:"id"`
	ForecastCollectionID     string    `json:"forecast_collection_id"`
	TargetTime               time.Time `json:"target_time"`
	IssuedAt                 time.Time `json:"issued_at"`
	ForecastHorizonMinutes   int       `json:"forecast_horizon_minutes"`
	TemperatureC             *float64  `json:"temperature_c,omitempty"`
	FeelsLikeTemperatureC    *float64  `json:"feels_like_temperature_c,omitempty"`
	PrecipitationProbability *float64  `json:"precipitation_probability,omitempty"`
	PrecipitationAmountMM    *float64  `json:"precipitation_amount_mm,omitempty"`
	HumidityPct              *float64  `json:"humidity_pct,omitempty"`
	WindSpeedMS              *float64  `json:"wind_speed_ms,omitempty"`
	WindDirectionDeg         *float64  `json:"wind_direction_deg,omitempty"`
	PressureHPa              *float64  `json:"pressure_hpa,omitempty"`
	CloudCoverPct            *float64  `json:"cloud_cover_pct,omitempty"`
	CanonicalConditionCode   string    `json:"canonical_condition_code,omitempty"`
}

func snapshotDTO(s *collectiondomain.ForecastSnapshot) SnapshotDTO {
	return SnapshotDTO{
		ID: s.ID.String(), ForecastCollectionID: s.ForecastCollectionID.String(),
		TargetTime: s.TargetTime, IssuedAt: s.IssuedAt, ForecastHorizonMinutes: s.ForecastHorizonMinutes,
		TemperatureC: s.TemperatureC, FeelsLikeTemperatureC: s.FeelsLikeTemperatureC,
		PrecipitationProbability: s.PrecipitationProbability, PrecipitationAmountMM: s.PrecipitationAmountMM,
		HumidityPct: s.HumidityPct, WindSpeedMS: s.WindSpeedMS, WindDirectionDeg: s.WindDirectionDeg,
		PressureHPa: s.PressureHPa, CloudCoverPct: s.CloudCoverPct,
		CanonicalConditionCode: s.CanonicalConditionCode,
	}
}

// ── Request bodies ────────────────────────────────────────────────────

// CreateLocationRequest is the POST /locations body.
type CreateLocationRequest struct {
	Name               string  `json:"name" binding:"required"`
	Latitude           float64 `json:"latitude" binding:"required"`
	Longitude          float64 `json:"longitude" binding:"required"`
	CountryCode        string  `json:"country_code" binding:"required"`
	Timezone           string  `json:"timezone" binding:"required"`
	AllowNearDuplicate bool    `json:"allow_near_duplicate"`
	OverrideReason     string  `json:"override_reason"`
}

// UpdateLocationRequest is the PUT /locations/{id} body. Only mutable fields
// (name); immutable fields are not accepted (domain architecture §2.3).
type UpdateLocationRequest struct {
	Name string `json:"name" binding:"required"`
}

// SetLocationStatusRequest is the PATCH /locations/{id}/status body.
type SetLocationStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

// TriggerCollectionRequest is the POST /admin/collections/trigger body.
type TriggerCollectionRequest struct {
	ProviderID string `json:"provider_id" binding:"required"`
	LocationID string `json:"location_id" binding:"required"`
}

// ── helpers ───────────────────────────────────────────────────────────

// pathUUID parses a UUID path parameter, writing a 422 problem on failure.
func pathUUID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		respond.Error(c, &badUUID{field: name}, respond.RequestID(c), c.Request.URL.Path)
		return uuid.Nil, false
	}
	return id, true
}

// queryUUID parses an optional UUID query parameter.
func queryUUID(c *gin.Context, name string) (*uuid.UUID, bool) {
	raw := c.Query(name)
	if raw == "" {
		return nil, true
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		respond.Error(c, &badUUID{field: name}, respond.RequestID(c), c.Request.URL.Path)
		return nil, false
	}
	return &id, true
}

// badUUID is a field validation error for malformed UUID parameters.
type badUUID struct{ field string }

func (e *badUUID) Error() string   { return e.field + ": must be a valid UUID" }
func (e *badUUID) Field() string   { return e.field }
func (e *badUUID) Message() string { return "must be a valid UUID" }

// fieldErr is a generic single-field validation error (query-param validation),
// mapped to a 422 problem by the respond layer's fieldMessage path.
type fieldErr struct{ field, message string }

func (e *fieldErr) Error() string   { return e.field + ": " + e.message }
func (e *fieldErr) Field() string   { return e.field }
func (e *fieldErr) Message() string { return e.message }

// parseUUIDParam parses a UUID string from a request field, returning a
// field-validation error naming the field on failure.
func parseUUIDParam(raw, field string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, &badUUID{field: field}
	}
	return id, nil
}

// actor builds an audit actor from the authenticated principal.
func actor(c *gin.Context) catalog.Actor {
	p, _ := respond.PrincipalFrom(c)
	return catalog.Actor{UserID: p.UserID, Name: p.Name, IPAddress: c.ClientIP()}
}
