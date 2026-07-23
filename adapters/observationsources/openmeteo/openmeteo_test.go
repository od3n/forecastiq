package openmeteo_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/adapters/observationsources/openmeteo"
	collectiondomain "github.com/forecastiq/forecastiq/internal/collection/domain"
	"github.com/forecastiq/forecastiq/internal/collection/ports"
)

// The fixture window is [09:00, 11:00] UTC on 2026-07-22 (a 2 h backfill window
// with three hourly rows).
var (
	windowStart = time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)
	windowEnd   = time.Date(2026, 7, 22, 11, 0, 0, 0, time.UTC)
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "test", "fixtures", "openmeteo-historical", name))
	require.NoError(t, err, "read fixture %s", name)
	return b
}

func serve(t *testing.T, body []byte, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newAdapter(t *testing.T) *openmeteo.Adapter {
	t.Helper()
	return openmeteo.New(openmeteo.Config{
		Client:         &http.Client{Timeout: 5 * time.Second},
		MaxRetries:     1,
		RetryBaseDelay: time.Millisecond,
	})
}

func newRequest(baseURL string) ports.ObservationRequest {
	return ports.ObservationRequest{
		LocationID:  uuid.MustParse("00000000-0000-0000-0000-000000000030"),
		Source:      openmeteo.Source,
		BaseURL:     baseURL,
		Latitude:    1.4927,
		Longitude:   103.7414,
		Timezone:    "Asia/Kuala_Lumpur",
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
	}
}

func TestFetch_Success(t *testing.T) {
	srv := serve(t, loadFixture(t, "historical_success_v1.json"), http.StatusOK)
	res, err := newAdapter(t).FetchObservations(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)

	assert.Equal(t, ports.OutcomeSuccess, res.Outcome)
	assert.Equal(t, 3, res.RecordsReceived)
	assert.Equal(t, 0, res.SuspectCount)
	require.Len(t, res.Observations, 3)
	assert.Equal(t, openmeteo.Source, res.Source)
	assert.Equal(t, openmeteo.SchemaVersion, res.SchemaVersion)

	o0 := res.Observations[0]
	assert.Equal(t, 30.5, *o0.TemperatureC)
	assert.Equal(t, 72.0, *o0.HumidityPct)
	assert.Equal(t, 0.0, *o0.PrecipitationMM)
	assert.Equal(t, collectiondomain.ObservationReanalysis, o0.ObservationType)
	assert.Equal(t, collectiondomain.QualityValid, o0.QualityFlag)
	assert.Equal(t, collectiondomain.ConditionPartlyCloudy, o0.CanonicalConditionCode) // WMO 2
	assert.Equal(t, "2", o0.ProviderConditionCode)
	assert.True(t, o0.ObservedAt.Equal(time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)))
	assert.Equal(t, "UTC", o0.ObservedAt.Location().String())
	assert.Equal(t, collectiondomain.ConditionRain, res.Observations[2].CanonicalConditionCode) // WMO 61
}

func TestFetch_EdgeNulls(t *testing.T) {
	srv := serve(t, loadFixture(t, "historical_edge_nulls.json"), http.StatusOK)
	res, err := newAdapter(t).FetchObservations(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)

	assert.Equal(t, ports.OutcomeSuccess, res.Outcome)
	require.Len(t, res.Observations, 2)
	assert.NotNil(t, res.Observations[0].TemperatureC)
	assert.Nil(t, res.Observations[0].HumidityPct)
	assert.Nil(t, res.Observations[0].PrecipitationMM)
	assert.Nil(t, res.Observations[1].TemperatureC)
	assert.Empty(t, res.Observations[1].CanonicalConditionCode, "null weather_code → no condition")
}

// TestFetch_Suspect proves OC-04 range violations flag the row suspect and keep
// it (workflow §5) — never dropped, counted for the suspect metric.
func TestFetch_Suspect(t *testing.T) {
	srv := serve(t, loadFixture(t, "historical_suspect.json"), http.StatusOK)
	res, err := newAdapter(t).FetchObservations(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)

	assert.Equal(t, ports.OutcomeSuccess, res.Outcome)
	assert.Equal(t, 3, res.RecordsReceived)
	require.Len(t, res.Observations, 3, "suspect rows are kept, not dropped")
	assert.Equal(t, 1, res.SuspectCount)
	assert.Equal(t, collectiondomain.QualitySuspect, res.Observations[1].QualityFlag)
	assert.Equal(t, collectiondomain.QualityValid, res.Observations[0].QualityFlag)
	assert.NotEmpty(t, res.InvalidReasons)
}

func TestFetch_SchemaDrift(t *testing.T) {
	srv := serve(t, loadFixture(t, "historical_schema_drift.json"), http.StatusOK)
	res, err := newAdapter(t).FetchObservations(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)

	assert.Equal(t, ports.OutcomeFailed, res.Outcome)
	assert.Equal(t, "schema_drift", res.ErrorCode)
	assert.Empty(t, res.Observations)
}

func TestFetch_ServerError(t *testing.T) {
	srv := serve(t, []byte(`{"error":"boom"}`), http.StatusInternalServerError)
	res, err := newAdapter(t).FetchObservations(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)
	assert.Equal(t, ports.OutcomeFailed, res.Outcome)
	assert.Equal(t, "provider_5xx", res.ErrorCode)
}

// TestFetch_ProvenanceDefault: Open-Meteo Historical exposes no per-variable
// provenance, so every row defaults to reanalysis (ADR-003 / decision A-4).
func TestFetch_ProvenanceDefault(t *testing.T) {
	srv := serve(t, loadFixture(t, "historical_success_v1.json"), http.StatusOK)
	res, err := newAdapter(t).FetchObservations(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)
	for _, o := range res.Observations {
		assert.Equal(t, collectiondomain.ObservationReanalysis, o.ObservationType)
	}
}

// TestFetch_ProvenanceOverride: the documented default is overridable via Config
// (ready for a future station/interpolated source).
func TestFetch_ProvenanceOverride(t *testing.T) {
	srv := serve(t, loadFixture(t, "historical_success_v1.json"), http.StatusOK)
	a := openmeteo.New(openmeteo.Config{
		Client:                 &http.Client{Timeout: 5 * time.Second},
		MaxRetries:             1,
		RetryBaseDelay:         time.Millisecond,
		DefaultObservationType: collectiondomain.ObservationStation,
	})
	res, err := a.FetchObservations(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)
	require.NotEmpty(t, res.Observations)
	assert.Equal(t, collectiondomain.ObservationStation, res.Observations[0].ObservationType)
}

// TestFetch_RequestShape proves the outgoing request pins UTC and the 2 h window
// (start_hour/end_hour) with the measured hourly variables.
func TestFetch_RequestShape(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(loadFixture(t, "historical_success_v1.json"))
	}))
	t.Cleanup(srv.Close)

	res, err := newAdapter(t).FetchObservations(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)
	assert.Equal(t, ports.OutcomeSuccess, res.Outcome)

	assert.Equal(t, "UTC", gotQuery.Get("timezone"))
	assert.Equal(t, "2026-07-22T09:00", gotQuery.Get("start_hour"))
	assert.Equal(t, "2026-07-22T11:00", gotQuery.Get("end_hour"))
	assert.Equal(t, "1.492700", gotQuery.Get("latitude"))
	assert.Contains(t, gotQuery.Get("hourly"), "temperature_2m")
	assert.Contains(t, gotQuery.Get("hourly"), "weather_code")
}

// TestFetch_FutureObservedAtRejected proves the observed_at ≤ window-end
// invariant (OC-04): a row beyond the requested window end is not built.
func TestFetch_FutureObservedAtRejected(t *testing.T) {
	srv := serve(t, loadFixture(t, "historical_success_v1.json"), http.StatusOK)
	req := newRequest(srv.URL)
	req.WindowEnd = time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC) // excludes the 11:00 row
	res, err := newAdapter(t).FetchObservations(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, 3, res.RecordsReceived)
	require.Len(t, res.Observations, 2, "the 11:00 row is beyond window end and rejected")
	assert.Equal(t, 1, res.InvalidCount)
	assert.True(t, res.Observations[1].ObservedAt.Equal(time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)))
}

// TestCorrectionDetection proves the value-diff-beyond-ε mechanism (workflow §4)
// against a re-fetched window: rows whose values changed are corrections; an
// unchanged row deduplicates (no correction).
func TestCorrectionDetection(t *testing.T) {
	a := newAdapter(t)
	srv1 := serve(t, loadFixture(t, "historical_success_v1.json"), http.StatusOK)
	first, err := a.FetchObservations(context.Background(), newRequest(srv1.URL))
	require.NoError(t, err)
	srv2 := serve(t, loadFixture(t, "historical_corrected.json"), http.StatusOK)
	second, err := a.FetchObservations(context.Background(), newRequest(srv2.URL))
	require.NoError(t, err)

	require.Len(t, first.Observations, 3)
	require.Len(t, second.Observations, 3)
	// Same observed_at ordering; compare position-wise (dedup key is observed_at).
	assert.True(t, second.Observations[0].DiffersFrom(first.Observations[0]), "09:00 temp 30.5→30.9 is a correction")
	assert.False(t, second.Observations[1].DiffersFrom(first.Observations[1]), "10:00 unchanged → dedup")
	assert.True(t, second.Observations[2].DiffersFrom(first.Observations[2]), "11:00 temp 31.8→32.4 is a correction")
}

// TestFetch_AuthFailed_NoRetry proves a non-retryable classification performs
// exactly one upstream request even with a retry budget (FC-08/FC-13). Historical
// is keyless, but the transport contract still holds for a 4xx.
func TestFetch_ClientError_NoRetry(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad request"}`))
	}))
	t.Cleanup(srv.Close)

	a := openmeteo.New(openmeteo.Config{
		Client:         &http.Client{Timeout: 5 * time.Second},
		MaxRetries:     3,
		RetryBaseDelay: time.Millisecond,
	})
	res, err := a.FetchObservations(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)
	assert.Equal(t, ports.OutcomeFailed, res.Outcome)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "non-retryable 4xx must not retry")
}
