package ports

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/collection/domain"
)

// ObservationRequest is the adapter input for one observation collection: a
// location and the [WindowStart, WindowEnd) UTC window to fetch (the 2 h
// backfill window; workflow §3). Open-Meteo Historical is keyless, so no
// credential is carried.
type ObservationRequest struct {
	LocationID  uuid.UUID
	Source      string
	BaseURL     string
	Latitude    float64
	Longitude   float64
	Timezone    string
	WindowStart time.Time // inclusive, UTC
	WindowEnd   time.Time // inclusive, UTC
}

// ObservationResult is the adapter output: the normalized/validated observation
// rows for the window plus response metadata and counts. Rows failing OC-04
// range checks are included with QualityFlag=suspect (never dropped; workflow
// §5); structurally unusable rows are counted in InvalidCount and omitted.
// Observations carry no raw-payload storage or checksum (ADR-025): the source
// is the re-queryable truth reference.
type ObservationResult struct {
	Observations    []*domain.Observation
	RecordsReceived int
	SuspectCount    int
	InvalidCount    int
	InvalidReasons  []string
	HTTPStatusCode  int
	LatencyMS       int
	Source          string
	SchemaVersion   string
	AdapterVersion  string
	RateLimit       *RateLimit
	Outcome         Outcome
	ErrorCode       string // classified (FC-13); e.g. schema_drift, timeout
	Err             error
}

// ObservationSourceAdapter fetches and normalizes one source's observations for
// a location-window. Implementations own transport hardening (via the shared
// providerhttp helper), schema validation, UTC normalization, provenance
// typing (observation_type), OC-04 range→suspect flagging, and condition
// mapping. Like the forecast port, a classified outcome returns (result, nil);
// a non-nil Go error is reserved for programmer errors. Adapters import this
// port, never the handlers (binding rule). Correction detection against
// already-stored rows is the caller's concern (domain.Observation.DiffersFrom).
type ObservationSourceAdapter interface {
	Source() string
	SchemaVersion() string
	AdapterVersion() string
	FetchObservations(ctx context.Context, req ObservationRequest) (*ObservationResult, error)
}
