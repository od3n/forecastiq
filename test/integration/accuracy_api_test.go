//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	analysisdomain "github.com/forecastiq/forecastiq/internal/analysis/domain"
	catalogdomain "github.com/forecastiq/forecastiq/internal/catalog/domain"
)

// TestAPI_AccuracySummaryLocationMode returns the per-provider metric grid,
// ranking status, and collection window for a location.
func TestAPI_AccuracySummaryLocationMode(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(3))
	e.seedCatalog(ctx, t)
	insertProviderX(ctx, t, e.pool)

	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)
	seedWorkedProvider(ctx, t, e.pool, catalogdomain.OpenMeteoProviderID, 1.20, 0.30, 0.769, 0.90, 1.10, 0.98, 0.99, 720, from, to)
	newRanker(e.pool).RankPeriod(ctx, analysisdomain.Period{Kind: analysisdomain.PeriodMonthly, Start: from, End: to})

	// Trigger a collection so collection_window (snapshot MIN/MAX) is populated.
	doRequest(e, http.MethodPost, "/api/v1/admin/collections/trigger", adminToken, map[string]any{
		"provider_id": catalogdomain.OpenMeteoProviderID.String(),
		"location_id": catalogdomain.JohorBahruLocationID.String(),
	})

	rec := doRequest(e, http.MethodGet,
		"/api/v1/accuracy/summary?location_id="+catalogdomain.JohorBahruLocationID.String()+"&horizon_minutes=1440", "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	env := decodeEnvelope(t, rec)
	data := env["data"].(map[string]any)
	assert.Equal(t, "location", data["mode"])
	providers := data["providers"].([]any)
	require.GreaterOrEqual(t, len(providers), 1)
	p := providers[0].(map[string]any)
	assert.Equal(t, "Open-Meteo", p["provider"].(map[string]any)["name"])
	assert.Equal(t, "ranked", p["ranking_status"])
	assert.NotEmpty(t, p["metrics"])
	window := p["collection_window"].(map[string]any)
	assert.NotNil(t, window["coverage"])
	assert.NotNil(t, window["last_snapshot_at"])
	assert.Equal(t, "public, max-age=60", rec.Header().Get("Cache-Control"))
	assert.Less(t, rec.Body.Len(), 40*1024) // size governance (caching §5)
}

// TestAPI_AccuracySummaryProviderMode returns a provider's ranking cells.
func TestAPI_AccuracySummaryProviderMode(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)
	seedWorkedProvider(ctx, t, e.pool, catalogdomain.OpenMeteoProviderID, 1.20, 0.30, 0.769, 0.90, 1.10, 0.98, 0.99, 720, from, to)
	newRanker(e.pool).RankPeriod(ctx, analysisdomain.Period{Kind: analysisdomain.PeriodMonthly, Start: from, End: to})

	rec := doRequest(e, http.MethodGet,
		"/api/v1/accuracy/summary?provider_id="+catalogdomain.OpenMeteoProviderID.String(), "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	env := decodeEnvelope(t, rec)
	data := env["data"].(map[string]any)
	assert.Equal(t, "provider", data["mode"])
	assert.Equal(t, "Open-Meteo", data["provider"].(map[string]any)["name"])
	cells := data["cells"].([]any)
	require.Len(t, cells, 1)
	cell := cells[0].(map[string]any)
	assert.Equal(t, "Johor Bahru", cell["location_name"])
	assert.EqualValues(t, 1440, cell["horizon_minutes"])
	assert.Equal(t, "ranked", cell["ranking_status"])
}

// TestAPI_AccuracySummaryValidation rejects zero-or-both selectors.
func TestAPI_AccuracySummaryValidation(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	assert.Equal(t, http.StatusUnprocessableEntity,
		doRequest(e, http.MethodGet, "/api/v1/accuracy/summary", "", nil).Code)
	assert.Equal(t, http.StatusUnprocessableEntity,
		doRequest(e, http.MethodGet, "/api/v1/accuracy/summary?location_id="+catalogdomain.JohorBahruLocationID.String()+
			"&provider_id="+catalogdomain.OpenMeteoProviderID.String(), "", nil).Code)
}

// TestAPI_AccuracyTrends buckets daily metric rows into a per-provider series
// with hollow points preserved.
func TestAPI_AccuracyTrends(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	base := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 4; i++ {
		day := base.AddDate(0, 0, i)
		var val *float64
		sample := 0
		if i != 2 { // day 2 is a hollow point (no data)
			v := 1.0 + float64(i)*0.1
			val, sample = &v, 24
		}
		insertAccuracyMetric(ctx, t, e.pool, catalogdomain.OpenMeteoProviderID,
			"temperature", "mae", val, sample, day, day.AddDate(0, 0, 1))
	}

	url := "/api/v1/accuracy?location_id=" + catalogdomain.JohorBahruLocationID.String() +
		"&horizon_minutes=1440&variable=temperature&metric_type=mae&aggregation=daily" +
		"&period_start=2026-07-10&period_end=2026-07-20"
	rec := doRequest(e, http.MethodGet, url, "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	env := decodeEnvelope(t, rec)
	data := env["data"].(map[string]any)
	assert.Equal(t, "daily", data["aggregation"])
	assert.Equal(t, "UTC", data["tz"])
	series := data["series"].([]any)
	require.Len(t, series, 1)
	buckets := series[0].(map[string]any)["buckets"].([]any)
	require.Len(t, buckets, 4)
	// The hollow point (index 2) carries a null value + sample_count 0.
	hollow := buckets[2].(map[string]any)
	assert.Nil(t, hollow["value"])
	assert.EqualValues(t, 0, hollow["sample_count"])
	assert.Less(t, rec.Body.Len(), 80*1024)
}

// TestAPI_AccuracyTrendsValidation rejects missing filters and over-range windows.
func TestAPI_AccuracyTrendsValidation(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)
	jb := catalogdomain.JohorBahruLocationID.String()

	// Missing variable/metric_type.
	assert.Equal(t, http.StatusUnprocessableEntity,
		doRequest(e, http.MethodGet, "/api/v1/accuracy?location_id="+jb, "", nil).Code)
	// Bad aggregation.
	assert.Equal(t, http.StatusUnprocessableEntity,
		doRequest(e, http.MethodGet, "/api/v1/accuracy?location_id="+jb+"&variable=temperature&metric_type=mae&aggregation=hourly", "", nil).Code)
	// Range > 365 days.
	assert.Equal(t, http.StatusUnprocessableEntity,
		doRequest(e, http.MethodGet, "/api/v1/accuracy?location_id="+jb+
			"&variable=temperature&metric_type=mae&period_start=2024-01-01&period_end=2026-01-01", "", nil).Code)
}
