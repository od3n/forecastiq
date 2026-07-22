// Package ports declares the collection module's contracts: the forecast
// provider adapter port (implemented by adapters/forecastproviders/*), the
// raw payload store port (adapters/payloadstore), and the persistence
// repository ports (adapters/persistence/collectionpg). Adapters import these
// ports, never the handlers (binding rule).
package ports

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/collection/domain"
)

// Outcome classifies a provider fetch result (drives collection status + FC-13
// error classification).
type Outcome string

const (
	OutcomeSuccess     Outcome = "success"
	OutcomePartial     Outcome = "partial"
	OutcomeFailed      Outcome = "failed"
	OutcomeRateLimited Outcome = "rate_limited"
	OutcomeTimeout     Outcome = "timeout"
	OutcomeAuthFailed  Outcome = "auth_failed"
)

// ForecastRequest is the adapter input for one collection.
type ForecastRequest struct {
	ProviderID   uuid.UUID
	LocationID   uuid.UUID
	ProviderSlug string
	BaseURL      string
	Credential   string // resolved secret value; empty when the provider needs none
	Latitude     float64
	Longitude    float64
	Timezone     string
	IssuedAt     time.Time // deterministic issuance computed by the service
}

// ForecastResult is the adapter output: raw payload + checksum (computed
// before parsing), response metadata, and normalized/validated snapshots.
// Snapshot IDs are precomputed (ADR-022); ForecastCollectionID is set by the
// service before persistence.
type ForecastResult struct {
	RawPayload         []byte
	Checksum           string
	HTTPStatusCode     int
	LatencyMS          int
	ProviderRequestID  string
	ModelRunTime       *time.Time
	SchemaVersion      string
	AdapterVersion     string
	IssuedAt           time.Time
	Snapshots          []*domain.ForecastSnapshot // valid rows only
	RecordsReceived    int
	InvalidCount       int
	InvalidReasons     []string
	UnmappedConditions map[string]int // provider condition codes with no canonical mapping (FC-15)
	RateLimit          *RateLimit     // normalized provider rate-limit metadata (nil when unsignalled)
	Outcome            Outcome
	ErrorCode          string // classified (FC-13); e.g. schema_drift, invalid_credentials
	Err                error  // underlying error for failed outcomes
}

// Capabilities declares what a provider adapter supports so the composition
// root, operators, and future scheduling can reason about a provider without
// provider-specific knowledge (architecture §2.4). Declared statically by each
// adapter; recorded in the registry.
type Capabilities struct {
	MaxForecastHorizon time.Duration // furthest-out prediction the adapter emits
	HourlyResolution   bool          // emits hourly (vs coarser) periods
	RequiresCredential bool          // needs a resolved credential to call
	SupportsReplay     bool          // implements ReplayDecoder (decode from stored bytes)
}

// ForecastProviderAdapter fetches and normalizes one provider's forecast.
// Implementations own retry, rate-limit awareness, schema validation, and
// condition mapping.
type ForecastProviderAdapter interface {
	Slug() string
	SchemaVersion() string
	AdapterVersion() string
	Capabilities() Capabilities
	FetchForecast(ctx context.Context, req ForecastRequest) (*ForecastResult, error)
}

// ReplayDecoder is an optional adapter capability: deterministically decode a
// previously stored raw payload into a ForecastResult WITHOUT any network call
// (domain §4.8 replay). Adapters that set Capabilities.SupportsReplay MUST
// implement it. The result carries no HTTP metadata (status/latency).
type ReplayDecoder interface {
	DecodeStored(ctx context.Context, req ForecastRequest, raw []byte) (*ForecastResult, error)
}
