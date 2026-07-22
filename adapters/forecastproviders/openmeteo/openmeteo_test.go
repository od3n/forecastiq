package openmeteo_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/adapters/forecastproviders/openmeteo"
	collectiondomain "github.com/forecastiq/forecastiq/internal/collection/domain"
	"github.com/forecastiq/forecastiq/internal/collection/ports"
)

// issuedAt is the deterministic issuance used across contract tests; fixture
// target times (11:00+) are strictly after it so horizons are positive.
var issuedAt = time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "test", "fixtures", "openmeteo", name))
	require.NoError(t, err, "read fixture %s", name)
	return b
}

// serve returns a test server that responds with the fixture body + status.
func serve(t *testing.T, body []byte, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newAdapter(t *testing.T, baseURL string) *openmeteo.Adapter {
	t.Helper()
	return openmeteo.New(openmeteo.Config{
		Client:         &http.Client{Timeout: 5 * time.Second},
		MaxRetries:     1,
		RetryBaseDelay: time.Millisecond,
	})
}

func newRequest(baseURL string) ports.ForecastRequest {
	return ports.ForecastRequest{
		ProviderID:   uuid.MustParse("00000000-0000-0000-0000-000000000010"),
		LocationID:   uuid.MustParse("00000000-0000-0000-0000-000000000030"),
		ProviderSlug: openmeteo.ProviderSlug,
		BaseURL:      baseURL,
		Latitude:     1.4927,
		Longitude:    103.7414,
		Timezone:     "Asia/Kuala_Lumpur",
		IssuedAt:     issuedAt,
	}
}

func TestFetch_Success(t *testing.T) {
	srv := serve(t, loadFixture(t, "forecast_success_v1.json"), http.StatusOK)
	a := newAdapter(t, srv.URL)
	res, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)

	assert.Equal(t, ports.OutcomeSuccess, res.Outcome)
	assert.Equal(t, 3, res.RecordsReceived)
	assert.Equal(t, 0, res.InvalidCount)
	require.Len(t, res.Snapshots, 3)
	assert.NotEmpty(t, res.Checksum)
	assert.Equal(t, openmeteo.SchemaVersion, res.SchemaVersion)

	s0 := res.Snapshots[0]
	assert.Equal(t, 31.2, *s0.TemperatureC)
	assert.Equal(t, 0.42, *s0.PrecipitationProbability) // 42% → 0.42
	assert.Equal(t, collectiondomain.ConditionCloudy, s0.CanonicalConditionCode)
	assert.Equal(t, "3", s0.ProviderConditionCode)
	assert.Equal(t, 60, s0.ForecastHorizonMinutes)
	assert.True(t, s0.TargetTime.Equal(time.Date(2026, 7, 22, 11, 0, 0, 0, time.UTC)))
	assert.True(t, s0.IssuedAt.Equal(issuedAt))
	assert.Equal(t, 180, res.Snapshots[2].ForecastHorizonMinutes)
}

func TestFetch_EdgeNulls(t *testing.T) {
	srv := serve(t, loadFixture(t, "forecast_edge_nulls.json"), http.StatusOK)
	a := newAdapter(t, srv.URL)
	res, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)

	assert.Equal(t, ports.OutcomeSuccess, res.Outcome)
	require.Len(t, res.Snapshots, 2)
	// Row 0: temperature present, apparent null. Row 1: temperature null.
	assert.NotNil(t, res.Snapshots[0].TemperatureC)
	assert.Nil(t, res.Snapshots[0].FeelsLikeTemperatureC)
	assert.Nil(t, res.Snapshots[1].TemperatureC)
	assert.Nil(t, res.Snapshots[1].PrecipitationProbability)
}

func TestFetch_PartialInvalid(t *testing.T) {
	srv := serve(t, loadFixture(t, "forecast_partial_invalid.json"), http.StatusOK)
	a := newAdapter(t, srv.URL)
	res, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)

	assert.Equal(t, ports.OutcomePartial, res.Outcome)
	assert.Equal(t, 4, res.RecordsReceived)
	assert.Equal(t, 1, res.InvalidCount) // 999°C row rejected
	require.Len(t, res.Snapshots, 3)
	assert.NotEmpty(t, res.InvalidReasons)
}

func TestFetch_SchemaDrift(t *testing.T) {
	srv := serve(t, loadFixture(t, "forecast_schema_drift.json"), http.StatusOK)
	a := newAdapter(t, srv.URL)
	res, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)

	assert.Equal(t, ports.OutcomeFailed, res.Outcome)
	assert.Equal(t, "schema_drift", res.ErrorCode)
	assert.Empty(t, res.Snapshots)
}

func TestFetch_UnmappedCondition(t *testing.T) {
	srv := serve(t, loadFixture(t, "forecast_unmapped_condition.json"), http.StatusOK)
	a := newAdapter(t, srv.URL)
	res, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)

	assert.Equal(t, ports.OutcomeSuccess, res.Outcome)
	require.Len(t, res.Snapshots, 2)
	assert.Equal(t, collectiondomain.ConditionUnknown, res.Snapshots[0].CanonicalConditionCode)
	assert.Equal(t, 1, res.UnmappedConditions["999"])
}

func TestFetch_RateLimited(t *testing.T) {
	srv := serve(t, []byte(`{"error":"too many requests"}`), http.StatusTooManyRequests)
	a := newAdapter(t, srv.URL)
	res, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)
	assert.Equal(t, ports.OutcomeRateLimited, res.Outcome)
}

func TestFetch_AuthFailed(t *testing.T) {
	srv := serve(t, []byte(`{"error":"unauthorized"}`), http.StatusUnauthorized)
	a := newAdapter(t, srv.URL)
	res, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)
	assert.Equal(t, ports.OutcomeAuthFailed, res.Outcome)
	assert.Equal(t, "invalid_credentials", res.ErrorCode)
}

func TestFetch_ServerError(t *testing.T) {
	srv := serve(t, []byte(`{"error":"boom"}`), http.StatusInternalServerError)
	a := newAdapter(t, srv.URL)
	res, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)
	assert.Equal(t, ports.OutcomeFailed, res.Outcome)
}

// TestReplayDeterminism: fetching the same payload twice yields identical
// checksums and snapshot counts (replay safety; domain §4.8).
func TestReplayDeterminism(t *testing.T) {
	body := loadFixture(t, "forecast_success_v1.json")
	srv := serve(t, body, http.StatusOK)
	a := newAdapter(t, srv.URL)

	res1, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)
	res2, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)

	assert.Equal(t, res1.Checksum, res2.Checksum)
	assert.Equal(t, len(res1.Snapshots), len(res2.Snapshots))
	assert.Equal(t, ports.Checksum(body), res1.Checksum)
}
