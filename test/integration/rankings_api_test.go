//go:build integration

package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	analysisdomain "github.com/forecastiq/forecastiq/internal/analysis/domain"
	catalogdomain "github.com/forecastiq/forecastiq/internal/catalog/domain"
	"github.com/forecastiq/forecastiq/internal/platform/ids"
)

// doRequestWithHeaders issues a GET with arbitrary headers (e.g. If-None-Match).
func doRequestWithHeaders(e *testEnv, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// TestAPI_Rankings serves the §8 worked-example cohort through GET /rankings and
// asserts the envelope shape, rank ordering (OM > OW > PX), observation context,
// versioning metadata, and ETag/304 caching.
func TestAPI_Rankings(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)
	insertProviderX(ctx, t, e.pool)

	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)
	seedWorkedProvider(ctx, t, e.pool, catalogdomain.OpenMeteoProviderID, 1.20, 0.30, 0.769, 0.90, 1.10, 0.98, 0.99, 720, from, to)
	seedWorkedProvider(ctx, t, e.pool, catalogdomain.OpenWeatherProviderID, 1.50, 0.90, 0.710, 1.40, 1.30, 0.92, 0.97, 700, from, to)
	seedWorkedProvider(ctx, t, e.pool, providerX, 1.10, 0.25, 0.682, 0.85, 1.60, 0.55, 0.90, 380, from, to)
	newRanker(e.pool).RankPeriod(ctx, analysisdomain.Period{Kind: analysisdomain.PeriodMonthly, Start: from, End: to})

	// A live observation drives observation_context (NP-01 provenance record).
	insertContextObservation(ctx, t, e.pool, 31.4, 0.0, time.Now().UTC().Add(-30*time.Minute))

	rec := doRequest(e, http.MethodGet,
		"/api/v1/rankings?location_id="+catalogdomain.JohorBahruLocationID.String()+"&horizon_minutes=1440", "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	env := decodeEnvelope(t, rec)
	data := env["data"].(map[string]any)
	rankings := data["rankings"].([]any)
	require.Len(t, rankings, 3)

	first := rankings[0].(map[string]any)
	assert.Equal(t, "Open-Meteo", first["provider"].(map[string]any)["name"])
	assert.EqualValues(t, 1, first["rank"])
	assert.Equal(t, "ranked", first["ranking_status"])
	assert.InDelta(t, 0.9568, first["composite_score"].(float64), 1e-3)
	assert.NotEmpty(t, first["component_scores"])

	third := rankings[2].(map[string]any)
	assert.Equal(t, "providerx", third["provider"].(map[string]any)["name"])
	assert.Equal(t, "provisionally_ranked", third["ranking_status"])
	assert.Equal(t, true, third["coverage_penalty_applied"])

	// Versioning metadata + attribution + observation context.
	meta := env["metadata"].(map[string]any)
	assert.Equal(t, "2026.1", meta["methodology_version"])
	assert.Equal(t, "w-2026.1", meta["weights_version"])
	assert.NotEmpty(t, env["attribution"])
	obs := data["observation_context"].(map[string]any)
	assert.InDelta(t, 31.4, obs["temperature_c"].(float64), 1e-9)
	assert.Equal(t, "reanalysis", obs["observation_type"])

	// Caching: strong ETag + conditional GET → 304.
	etag := rec.Header().Get("ETag")
	require.NotEmpty(t, etag)
	assert.Equal(t, "public, max-age=60", rec.Header().Get("Cache-Control"))
	req := doRequestWithHeaders(e, http.MethodGet,
		"/api/v1/rankings?location_id="+catalogdomain.JohorBahruLocationID.String()+"&horizon_minutes=1440",
		map[string]string{"If-None-Match": etag})
	assert.Equal(t, http.StatusNotModified, req.Code)

	// Response size bound (< 16 KB; caching doc §5).
	assert.Less(t, rec.Body.Len(), 16*1024)
}

// TestAPI_RankingsPartialResult surfaces an active provider that has no ranking
// for the cell as a provider_unavailable warning (omitted from rankings[]).
func TestAPI_RankingsPartialResult(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)
	// OpenWeather is an active provider with no ranking for this cell.
	insertProvider(ctx, t, e.pool, catalogdomain.OpenWeatherProviderID, "openweather-test")

	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)
	seedWorkedProvider(ctx, t, e.pool, catalogdomain.OpenMeteoProviderID, 1.20, 0.30, 0.769, 0.90, 1.10, 0.98, 0.99, 720, from, to)
	newRanker(e.pool).RankPeriod(ctx, analysisdomain.Period{Kind: analysisdomain.PeriodMonthly, Start: from, End: to})

	rec := doRequest(e, http.MethodGet,
		"/api/v1/rankings?location_id="+catalogdomain.JohorBahruLocationID.String()+"&horizon_minutes=1440", "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	env := decodeEnvelope(t, rec)
	rankings := env["data"].(map[string]any)["rankings"].([]any)
	assert.Len(t, rankings, 1) // only Open-Meteo has a ranking
	assert.Equal(t, true, env["partial_result"])
	warnings := env["warnings"].([]any)
	require.Len(t, warnings, 1)
	w := warnings[0].(map[string]any)
	assert.Equal(t, catalogdomain.OpenWeatherProviderID.String(), w["provider_id"])
	assert.Equal(t, "provider_unavailable", w["code"])
}

// TestAPI_RankingsUnrankedLowSamples serves a cohort where one provider has
// < 10 pairs AND coverage below the 0.5 floor: its row is unranked (rank
// omitted, composite null) but still carries sample_count + coverage, so the
// UI attributes the status to samples ("Insufficient data (5/30)") rather
// than the coverage message.
func TestAPI_RankingsUnrankedLowSamples(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)
	insertProvider(ctx, t, e.pool, catalogdomain.OpenWeatherProviderID, "openweather-test")

	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)
	seedWorkedProvider(ctx, t, e.pool, catalogdomain.OpenMeteoProviderID, 1.20, 0.30, 0.769, 0.90, 1.10, 0.98, 0.99, 720, from, to)
	// 5 pairs (< 10 provisional floor) and coverage 0.30 (< 0.5): the sample
	// floor is the trigger the UI must surface (§7.2 / BR-RANK-02).
	seedWorkedProvider(ctx, t, e.pool, catalogdomain.OpenWeatherProviderID, 1.50, 0.90, 0.710, 1.40, 1.30, 0.30, 0.97, 5, from, to)
	newRanker(e.pool).RankPeriod(ctx, analysisdomain.Period{Kind: analysisdomain.PeriodMonthly, Start: from, End: to})

	rec := doRequest(e, http.MethodGet,
		"/api/v1/rankings?location_id="+catalogdomain.JohorBahruLocationID.String()+"&horizon_minutes=1440", "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	env := decodeEnvelope(t, rec)
	data := env["data"].(map[string]any)
	assert.EqualValues(t, 30, data["min_sample_count"])
	rankings := data["rankings"].([]any)
	require.Len(t, rankings, 2)

	// Unranked rows sort last: no rank, no score ordering published.
	row := rankings[1].(map[string]any)
	assert.Equal(t, "openweather-test", row["provider"].(map[string]any)["name"])
	assert.Equal(t, "unranked", row["ranking_status"])
	_, hasRank := row["rank"]
	assert.False(t, hasRank, "unranked rows omit rank")
	assert.Nil(t, row["composite_score"])
	// The badge inputs: sample_count 5 (→ "Insufficient data (5/30)") with
	// coverage still present so the UI can tell samples are the trigger.
	assert.EqualValues(t, 5, row["sample_count"])
	assert.InDelta(t, 0.30, row["coverage"].(float64), 1e-9)
}

// TestAPI_RankingsValidation rejects a missing/invalid location_id with 422.
func TestAPI_RankingsValidation(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	rec := doRequest(e, http.MethodGet, "/api/v1/rankings", "", nil)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	rec = doRequest(e, http.MethodGet, "/api/v1/rankings?location_id=not-a-uuid", "", nil)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	rec = doRequest(e, http.MethodGet,
		"/api/v1/rankings?location_id="+catalogdomain.JohorBahruLocationID.String()+"&horizon_profile=bogus", "", nil)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

// TestAPI_RankingsMethodology returns the static methodology document.
func TestAPI_RankingsMethodology(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))

	rec := doRequest(e, http.MethodGet, "/api/v1/rankings/methodology", "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	env := decodeEnvelope(t, rec)
	data := env["data"].(map[string]any)
	assert.Equal(t, "2026.1", data["methodology_version"])
	assert.Equal(t, "w-2026.1", data["weights_version"])
	assert.NotEmpty(t, data["formulas"])
	assert.NotEmpty(t, data["default_weights"])
	thresholds := data["thresholds"].(map[string]any)
	assert.EqualValues(t, 30, thresholds["ranked"])
}

// insertObservation stores one live reanalysis observation for the JB location
// (ensuring the observed_at month's partition exists first).
func insertContextObservation(ctx context.Context, t *testing.T, pool *pgxpool.Pool, tempC, precipMM float64, at time.Time) {
	t.Helper()
	at = at.UTC()
	monthStart := time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, time.UTC)
	_, err := pool.Exec(ctx, `SELECT create_monthly_partition('observations', $1::date)`, monthStart)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO observations (id, location_id, source, observation_type, observed_at,
		   temperature_c, precipitation_mm, quality_flag)
		 VALUES ($1, $2, 'openmeteo_historical', 'reanalysis', $3, $4, $5, 'valid')`,
		ids.New(), catalogdomain.JohorBahruLocationID, at, tempC, precipMM)
	require.NoError(t, err)
}
