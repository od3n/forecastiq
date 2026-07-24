//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/adapters/persistence/collectionpg"
	catalogdomain "github.com/forecastiq/forecastiq/internal/catalog/domain"
	collectiondomain "github.com/forecastiq/forecastiq/internal/collection/domain"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
	"github.com/forecastiq/forecastiq/internal/platform/ids"
)

// seedSnapshot inserts one forecast_snapshots row (with its owning collection)
// at a fixed target_time + horizon for the seeded Open-Meteo provider/config at
// the JB location, ensuring the target month's partition exists.
func seedSnapshot(ctx context.Context, t *testing.T, e *testEnv, targetTime time.Time, horizon int, tempC float64) {
	t.Helper()
	targetTime = targetTime.UTC()
	collRepo := collectionpg.NewCollectionRepository()
	snapRepo := collectionpg.NewSnapshotRepository()
	issued := targetTime.Add(-time.Duration(horizon) * time.Minute)
	coll := &collectiondomain.ForecastCollection{
		ID: ids.New(), ProviderID: catalogdomain.OpenMeteoProviderID, LocationID: catalogdomain.JohorBahruLocationID,
		ProviderConfigurationID: catalogdomain.OpenMeteoConfigID, RequestedAt: issued,
		Status: collectiondomain.StatusSuccess, CreatedAt: issued,
	}
	require.NoError(t, collRepo.Insert(ctx, e.pool, coll))

	monthStart := time.Date(targetTime.Year(), targetTime.Month(), 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, e.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		return snapRepo.EnsurePartitions(ctx, tx, []time.Time{monthStart})
	}))
	temp := tempC
	snap := &collectiondomain.ForecastSnapshot{
		ID: ids.New(), ForecastCollectionID: coll.ID,
		ProviderID: catalogdomain.OpenMeteoProviderID, LocationID: catalogdomain.JohorBahruLocationID,
		IssuedAt: issued, TargetTime: targetTime, ForecastHorizonMinutes: horizon, TemperatureC: &temp,
	}
	require.NoError(t, e.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		_, err := snapRepo.InsertBatch(ctx, tx, []*collectiondomain.ForecastSnapshot{snap})
		return err
	}))
}

// TestAPI_ForecastComparison exercises the S-05 FvA endpoint end to end: per-hour
// forecast points at the requested horizon, observations (with a gap), in-memory
// day metrics reusing the WP-12 kernel, and provenance/freshness.
func TestAPI_ForecastComparison(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	// Date 2026-07-21 in Asia/Kuala_Lumpur (UTC+8) → UTC [07-20 16:00, 07-21 16:00).
	// Place three hourly target times inside that window.
	h0 := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	h1 := h0.Add(1 * time.Hour)
	h2 := h0.Add(2 * time.Hour)
	seedSnapshot(ctx, t, e, h0, 1440, 30.0)
	seedSnapshot(ctx, t, e, h1, 1440, 32.0)
	seedSnapshot(ctx, t, e, h2, 1440, 33.0)
	insertContextObservation(ctx, t, e.pool, 31.0, 0.0, h0)
	insertContextObservation(ctx, t, e.pool, 31.0, 0.0, h1)
	// h2 has no observation → a gap (absent from observations[], excluded from metrics).

	url := "/api/v1/forecast-comparison?location_id=" + catalogdomain.JohorBahruLocationID.String() +
		"&date=2026-07-21&variable=temperature&horizon_minutes=1440"
	rec := doRequest(e, http.MethodGet, url, "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	env := decodeEnvelope(t, rec)
	data := env["data"].(map[string]any)
	assert.Equal(t, "temperature", data["variable"])
	assert.Equal(t, "Asia/Kuala_Lumpur", data["location"].(map[string]any)["timezone"])
	assert.Equal(t, true, data["observations_available"])

	series := data["series"].([]any)
	require.Len(t, series, 1)
	points := series[0].(map[string]any)["points"].([]any)
	assert.Len(t, points, 3) // full forecast line incl. the gap hour
	observations := data["observations"].([]any)
	assert.Len(t, observations, 2) // gap hour absent

	metrics := data["day_metrics"].([]any)
	require.Len(t, metrics, 1)
	dm := metrics[0].(map[string]any)
	assert.EqualValues(t, 2, dm["sample_count"]) // only the two matched hours
	assert.InDelta(t, 1.0, dm["mae"].(float64), 1e-9)
	assert.Equal(t, "2026.1", dm["methodology_version"])
	assert.InDelta(t, 1.0, data["error_band_mae"].(float64), 1e-9)

	prov := env["provenance"].(map[string]any)["observation_provenance_mix"].(map[string]any)
	assert.InDelta(t, 1.0, prov["reanalysis"].(float64), 1e-9)
	assert.NotEmpty(t, env["attribution"])

	// Size bound (< 20 KB; caching §5) + ETag present.
	assert.Less(t, rec.Body.Len(), 20*1024)
	assert.NotEmpty(t, rec.Header().Get("ETag"))
}

// TestAPI_ForecastComparisonValidation covers required-param + 404 behaviour.
func TestAPI_ForecastComparisonValidation(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)
	jb := catalogdomain.JohorBahruLocationID.String()

	// Missing variable.
	assert.Equal(t, http.StatusUnprocessableEntity,
		doRequest(e, http.MethodGet, "/api/v1/forecast-comparison?location_id="+jb+"&date=2026-07-21&horizon_minutes=1440", "", nil).Code)
	// Bad variable.
	assert.Equal(t, http.StatusUnprocessableEntity,
		doRequest(e, http.MethodGet, "/api/v1/forecast-comparison?location_id="+jb+"&date=2026-07-21&variable=bogus&horizon_minutes=1440", "", nil).Code)
	// Missing horizon.
	assert.Equal(t, http.StatusUnprocessableEntity,
		doRequest(e, http.MethodGet, "/api/v1/forecast-comparison?location_id="+jb+"&date=2026-07-21&variable=temperature", "", nil).Code)
	// Bad date.
	assert.Equal(t, http.StatusUnprocessableEntity,
		doRequest(e, http.MethodGet, "/api/v1/forecast-comparison?location_id="+jb+"&date=nope&variable=temperature&horizon_minutes=1440", "", nil).Code)
	// Unknown location → 404.
	assert.Equal(t, http.StatusNotFound,
		doRequest(e, http.MethodGet, "/api/v1/forecast-comparison?location_id="+mustUUIDv7().String()+"&date=2026-07-21&variable=temperature&horizon_minutes=1440", "", nil).Code)
}
