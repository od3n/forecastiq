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
	Outcome            Outcome
	ErrorCode          string // classified (FC-13); e.g. schema_drift, invalid_credentials
	Err                error  // underlying error for failed outcomes
}

// ForecastProviderAdapter fetches and normalizes one provider's forecast.
// Implementations own retry, rate-limit awareness, schema validation, and
// condition mapping.
type ForecastProviderAdapter interface {
	Slug() string
	SchemaVersion() string
	AdapterVersion() string
	FetchForecast(ctx context.Context, req ForecastRequest) (*ForecastResult, error)
}
