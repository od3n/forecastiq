package openweather_test

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

	"github.com/forecastiq/forecastiq/adapters/forecastproviders/openweather"
	collectiondomain "github.com/forecastiq/forecastiq/internal/collection/domain"
	"github.com/forecastiq/forecastiq/internal/collection/ports"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
)

// issuedAt is the deterministic issuance used across contract tests; fixture
// target times (11:00+) are strictly after it so horizons are positive.
var issuedAt = time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "test", "fixtures", "openweather", name))
	require.NoError(t, err, "read fixture %s", name)
	return b
}

// serve returns a test server that responds with the fixture body + status.
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

func newAdapter(t *testing.T) *openweather.Adapter {
	t.Helper()
	return openweather.New(openweather.Config{
		Client:         &http.Client{Timeout: 5 * time.Second},
		MaxRetries:     1,
		RetryBaseDelay: time.Millisecond,
	})
}

func newRequest(baseURL string) ports.ForecastRequest {
	return ports.ForecastRequest{
		ProviderID:   uuid.MustParse("00000000-0000-0000-0000-000000000011"),
		LocationID:   uuid.MustParse("00000000-0000-0000-0000-000000000030"),
		ProviderSlug: openweather.ProviderSlug,
		BaseURL:      baseURL,
		Credential:   "test-api-key",
		Latitude:     1.4927,
		Longitude:    103.7414,
		Timezone:     "Asia/Kuala_Lumpur",
		IssuedAt:     issuedAt,
	}
}

func TestFetch_Success(t *testing.T) {
	srv := serve(t, loadFixture(t, "onecall_success_v3.json"), http.StatusOK)
	a := newAdapter(t)
	res, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)

	assert.Equal(t, ports.OutcomeSuccess, res.Outcome)
	assert.Equal(t, 3, res.RecordsReceived)
	assert.Equal(t, 0, res.InvalidCount)
	require.Len(t, res.Snapshots, 3)
	assert.NotEmpty(t, res.Checksum)
	assert.Equal(t, openweather.SchemaVersion, res.SchemaVersion)

	s0 := res.Snapshots[0]
	assert.Equal(t, 31.2, *s0.TemperatureC)
	assert.Equal(t, 0.42, *s0.PrecipitationProbability) // already [0,1] — no conversion
	assert.Equal(t, 0.5, *s0.PrecipitationAmountMM)     // rain.1h
	assert.Equal(t, collectiondomain.ConditionCloudy, s0.CanonicalConditionCode)
	assert.Equal(t, "803", s0.ProviderConditionCode)
	assert.Equal(t, 60, s0.ForecastHorizonMinutes)
	assert.True(t, s0.TargetTime.Equal(time.Date(2026, 7, 22, 11, 0, 0, 0, time.UTC)))
	assert.True(t, s0.IssuedAt.Equal(issuedAt))
	assert.Equal(t, 180, res.Snapshots[2].ForecastHorizonMinutes)
	assert.Equal(t, collectiondomain.ConditionClear, res.Snapshots[2].CanonicalConditionCode)
}

func TestFetch_EdgeNulls(t *testing.T) {
	srv := serve(t, loadFixture(t, "onecall_edge_nulls.json"), http.StatusOK)
	a := newAdapter(t)
	res, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)

	assert.Equal(t, ports.OutcomeSuccess, res.Outcome)
	require.Len(t, res.Snapshots, 2)
	// Row 0: temperature present, feels_like absent. Row 1: temperature absent.
	assert.NotNil(t, res.Snapshots[0].TemperatureC)
	assert.Nil(t, res.Snapshots[0].FeelsLikeTemperatureC)
	assert.Nil(t, res.Snapshots[0].PrecipitationProbability) // pop absent (dry period)
	assert.Nil(t, res.Snapshots[0].PrecipitationAmountMM)
	assert.Nil(t, res.Snapshots[1].TemperatureC)
}

func TestFetch_PartialInvalid(t *testing.T) {
	srv := serve(t, loadFixture(t, "onecall_partial_invalid.json"), http.StatusOK)
	a := newAdapter(t)
	res, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)

	assert.Equal(t, ports.OutcomePartial, res.Outcome)
	assert.Equal(t, 4, res.RecordsReceived)
	assert.Equal(t, 1, res.InvalidCount) // 999°C row rejected
	require.Len(t, res.Snapshots, 3)
	assert.NotEmpty(t, res.InvalidReasons)
}

func TestFetch_SchemaDrift(t *testing.T) {
	srv := serve(t, loadFixture(t, "onecall_schema_drift.json"), http.StatusOK)
	a := newAdapter(t)
	res, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)

	assert.Equal(t, ports.OutcomeFailed, res.Outcome)
	assert.Equal(t, "schema_drift", res.ErrorCode)
	assert.Empty(t, res.Snapshots)
}

// TestFetch_SchemaDrift_MajorityInvalid proves the >50%-invalid branch: a
// structurally valid payload whose majority of rows are out of range ⇒ failed
// + schema_drift (the lone valid row is still decomposed).
func TestFetch_SchemaDrift_MajorityInvalid(t *testing.T) {
	srv := serve(t, loadFixture(t, "onecall_majority_invalid.json"), http.StatusOK)
	a := newAdapter(t)
	res, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)

	assert.Equal(t, ports.OutcomeFailed, res.Outcome)
	assert.Equal(t, "schema_drift", res.ErrorCode)
	assert.Equal(t, 4, res.RecordsReceived)
	assert.Equal(t, 3, res.InvalidCount)
	assert.Len(t, res.Snapshots, 1)
}

func TestFetch_UnmappedCondition(t *testing.T) {
	srv := serve(t, loadFixture(t, "onecall_unmapped_condition.json"), http.StatusOK)
	a := newAdapter(t)
	res, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)

	assert.Equal(t, ports.OutcomeSuccess, res.Outcome)
	require.Len(t, res.Snapshots, 2)
	assert.Equal(t, collectiondomain.ConditionUnknown, res.Snapshots[0].CanonicalConditionCode)
	assert.Equal(t, 1, res.UnmappedConditions["781"]) // tornado has no canonical equivalent
}

func TestFetch_RateLimited(t *testing.T) {
	srv := serveRateLimited(t)
	a := newAdapter(t)
	res, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)
	assert.Equal(t, ports.OutcomeRateLimited, res.Outcome)
	assert.Equal(t, "rate_limited", res.ErrorCode)
	require.NotNil(t, res.RateLimit)
	require.NotNil(t, res.RateLimit.RetryAfter)
	assert.Equal(t, 60*time.Second, *res.RateLimit.RetryAfter)
}

// serveRateLimited returns a 429 with a Retry-After header and the recorded
// OpenWeather 429 body.
func serveRateLimited(t *testing.T) *httptest.Server {
	t.Helper()
	body := loadFixture(t, "onecall_429.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFetch_AuthFailed(t *testing.T) {
	srv := serve(t, loadFixture(t, "onecall_401.json"), http.StatusUnauthorized)
	a := newAdapter(t)
	res, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)
	assert.Equal(t, ports.OutcomeAuthFailed, res.Outcome)
	assert.Equal(t, "invalid_credentials", res.ErrorCode)
}

// TestFetch_AuthFailed_NoRetry proves 401 is terminal (FC-08/FC-13): even with
// a retry budget, an invalid-credentials classification performs exactly one
// upstream request.
func TestFetch_AuthFailed_NoRetry(t *testing.T) {
	var calls int32
	body := loadFixture(t, "onecall_401.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	a := openweather.New(openweather.Config{
		Client:         &http.Client{Timeout: 5 * time.Second},
		MaxRetries:     3, // budget retries; a 401 must still call exactly once
		RetryBaseDelay: time.Millisecond,
	})
	res, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)
	assert.Equal(t, ports.OutcomeAuthFailed, res.Outcome)
	assert.Equal(t, "invalid_credentials", res.ErrorCode)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestFetch_ServerError(t *testing.T) {
	srv := serve(t, []byte(`{"cod":500,"message":"boom"}`), http.StatusInternalServerError)
	a := newAdapter(t)
	res, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)
	assert.Equal(t, ports.OutcomeFailed, res.Outcome)
	assert.Equal(t, "provider_5xx", res.ErrorCode)
}

func TestCapabilities(t *testing.T) {
	a := newAdapter(t)
	caps := a.Capabilities()
	assert.Equal(t, 48*time.Hour, caps.MaxForecastHorizon)
	assert.True(t, caps.HourlyResolution)
	assert.True(t, caps.RequiresCredential)
	assert.True(t, caps.SupportsReplay)
	// The adapter must satisfy the optional replay interface it advertises.
	_, ok := interface{}(a).(ports.ReplayDecoder)
	assert.True(t, ok)
	assert.Equal(t, openweather.ProviderSlug, a.Slug())
	assert.Equal(t, openweather.SchemaVersion, a.SchemaVersion())
	assert.Equal(t, openweather.AdapterVersion, a.AdapterVersion())
}

// TestFetch_RequestShape proves the outgoing request pins UTC-safe canonical
// units and passes the credential as the appid query parameter (contract
// matrix §1.2 timezone/attribution rows; security §8 — appid never logged).
func TestFetch_RequestShape(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(loadFixture(t, "onecall_success_v3.json"))
	}))
	t.Cleanup(srv.Close)

	a := newAdapter(t)
	res, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)

	assert.Equal(t, "metric", gotQuery.Get("units"))
	assert.Equal(t, "current,minutely,daily,alerts", gotQuery.Get("exclude"))
	assert.Equal(t, "test-api-key", gotQuery.Get("appid"))
	assert.Equal(t, "1.492700", gotQuery.Get("lat"))
	assert.Equal(t, "103.741400", gotQuery.Get("lon"))

	// dt epochs normalize to the exact UTC instant regardless of the response's
	// timezone_offset (BR-PROV-01); no local-time skew is applied.
	require.Len(t, res.Snapshots, 3)
	s0 := res.Snapshots[0]
	assert.True(t, s0.TargetTime.Equal(time.Date(2026, 7, 22, 11, 0, 0, 0, time.UTC)))
	assert.Equal(t, "UTC", s0.TargetTime.Location().String())
}

// TestFetch_AttributionFields proves the adapter captures the provider request
// id when exposed and never fabricates a model-run time (OneCall exposes none).
func TestFetch_AttributionFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "ow-req-xyz789")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(loadFixture(t, "onecall_success_v3.json"))
	}))
	t.Cleanup(srv.Close)

	a := newAdapter(t)
	res, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)

	assert.Equal(t, ports.OutcomeSuccess, res.Outcome)
	assert.Equal(t, "ow-req-xyz789", res.ProviderRequestID)
	assert.Nil(t, res.ModelRunTime)
}

// TestDecodeStored_ReplayDeterminism: DecodeStored re-derives an identical
// result from stored bytes with NO network call (domain §4.8).
func TestDecodeStored_ReplayDeterminism(t *testing.T) {
	body := loadFixture(t, "onecall_success_v3.json")
	a := newAdapter(t)

	res1, err := a.DecodeStored(context.Background(), newRequest(""), body)
	require.NoError(t, err)
	res2, err := a.DecodeStored(context.Background(), newRequest(""), body)
	require.NoError(t, err)

	assert.Equal(t, ports.OutcomeSuccess, res1.Outcome)
	assert.Equal(t, ports.Checksum(body), res1.Checksum)
	assert.Equal(t, res1.Checksum, res2.Checksum)
	assert.Equal(t, len(res1.Snapshots), len(res2.Snapshots))
	// Replay carries no HTTP metadata.
	assert.Zero(t, res1.HTTPStatusCode)
	assert.Zero(t, res1.LatencyMS)
}

// TestReplayDeterminism: fetching the same payload twice yields identical
// checksums and snapshot counts (replay safety; domain §4.8).
func TestReplayDeterminism(t *testing.T) {
	body := loadFixture(t, "onecall_success_v3.json")
	srv := serve(t, body, http.StatusOK)
	a := newAdapter(t)

	res1, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)
	res2, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)

	assert.Equal(t, res1.Checksum, res2.Checksum)
	assert.Equal(t, len(res1.Snapshots), len(res2.Snapshots))
	assert.Equal(t, ports.Checksum(body), res1.Checksum)
}

// TestFetch_BudgetExhausted_NoUpstreamCall proves the daily rate-budget guard
// (WP-07): once the configured daily budget is spent, further collections are
// refused pre-emptively with rate_limited and make NO upstream request.
func TestFetch_BudgetExhausted_NoUpstreamCall(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(loadFixture(t, "onecall_success_v3.json"))
	}))
	t.Cleanup(srv.Close)

	a := openweather.New(openweather.Config{
		Client:         &http.Client{Timeout: 5 * time.Second},
		MaxRetries:     1,
		RetryBaseDelay: time.Millisecond,
		DailyBudget:    2,
		Clock:          clock.Fixed{T: time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)},
	})

	for i := 0; i < 2; i++ {
		res, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
		require.NoError(t, err)
		assert.Equal(t, ports.OutcomeSuccess, res.Outcome)
	}
	// Third call exceeds the budget: refused without an upstream request.
	res, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)
	assert.Equal(t, ports.OutcomeRateLimited, res.Outcome)
	assert.Equal(t, "rate_limited", res.ErrorCode)
	require.NotNil(t, res.RateLimit)
	require.NotNil(t, res.RateLimit.RetryAfter)
	assert.Equal(t, int32(2), atomic.LoadInt32(&calls), "budget must not call upstream once spent")
}

// TestFetch_RateLimit_EngagesPause proves 429 → pause: an upstream 429 pauses
// the adapter so the next slot is refused WITHOUT an upstream call, and
// collection resumes once the Retry-After window elapses.
func TestFetch_RateLimit_EngagesPause(t *testing.T) {
	var calls int32
	body429 := loadFixture(t, "onecall_429.json")
	bodyOK := loadFixture(t, "onecall_success_v3.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write(body429)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(bodyOK)
	}))
	t.Cleanup(srv.Close)

	clk := clock.NewMutable(time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC))
	a := openweather.New(openweather.Config{
		Client:         &http.Client{Timeout: 5 * time.Second},
		MaxRetries:     1, // no transport retry; the 429 surfaces immediately
		RetryBaseDelay: time.Millisecond,
		DailyBudget:    1000,
		Clock:          clk,
	})

	// 1) Upstream 429 → rate_limited; pause engaged.
	res, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)
	assert.Equal(t, ports.OutcomeRateLimited, res.Outcome)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))

	// 2) Within the pause window → refused pre-emptively, no upstream call.
	res, err = a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)
	assert.Equal(t, ports.OutcomeRateLimited, res.Outcome)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "paused adapter must not call upstream")

	// 3) After Retry-After elapses → resumes and succeeds.
	clk.Advance(31 * time.Second)
	res, err = a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)
	assert.Equal(t, ports.OutcomeSuccess, res.Outcome)
	assert.Equal(t, int32(2), atomic.LoadInt32(&calls))
}

// TestFetch_RateLimited_NoRetry proves a daily-quota 429 triggers exactly ONE
// upstream request even with a retry budget (DRB-WP07-001): retrying a spent
// daily allowance only burns quota, so the adapter opts 429 out of transport
// retry and lets the budget guard pause instead.
func TestFetch_RateLimited_NoRetry(t *testing.T) {
	var calls int32
	body429 := loadFixture(t, "onecall_429.json")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write(body429)
	}))
	t.Cleanup(srv.Close)

	a := openweather.New(openweather.Config{
		Client:         &http.Client{Timeout: 5 * time.Second},
		MaxRetries:     3, // budget retries; a 429 must still call exactly once
		RetryBaseDelay: time.Millisecond,
		DailyBudget:    1000,
		Clock:          clock.Fixed{T: time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)},
	})
	res, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)
	assert.Equal(t, ports.OutcomeRateLimited, res.Outcome)
	assert.Equal(t, "rate_limited", res.ErrorCode)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "daily-quota 429 must not be retried")
}

// TestFetch_RetriesCountAgainstBudget proves transport-level FC-08 retries are
// debited from the daily budget (DRB-WP07-001): a single failing collection
// that the transport retries consumes one budget unit per actual upstream
// request, so the budget reflects real provider traffic, not attempts. With a
// budget of 3 and a 3-attempt 5xx storm, the day's budget is fully spent and
// the next collection is refused pre-emptively (no further upstream call).
func TestFetch_RetriesCountAgainstBudget(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusServiceUnavailable) // retryable 5xx
		_, _ = w.Write([]byte(`{"cod":503,"message":"unavailable"}`))
	}))
	t.Cleanup(srv.Close)

	a := openweather.New(openweather.Config{
		Client:         &http.Client{Timeout: 5 * time.Second},
		MaxRetries:     3, // one collection == up to 3 upstream requests
		RetryBaseDelay: time.Millisecond,
		DailyBudget:    3,
		Clock:          clock.Fixed{T: time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)},
	})

	// Fetch #1: 1 admitted + 2 retries accounted == 3 upstream calls == budget spent.
	res, err := a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)
	assert.Equal(t, ports.OutcomeFailed, res.Outcome)
	assert.Equal(t, "provider_5xx", res.ErrorCode)
	assert.Equal(t, int32(3), atomic.LoadInt32(&calls), "5xx retried up to MaxRetries")

	// Fetch #2: budget fully spent by the retries → refused pre-emptively.
	res, err = a.FetchForecast(context.Background(), newRequest(srv.URL))
	require.NoError(t, err)
	assert.Equal(t, ports.OutcomeRateLimited, res.Outcome)
	assert.Equal(t, int32(3), atomic.LoadInt32(&calls), "retries must be debited so the budget refuses the next call")
}
