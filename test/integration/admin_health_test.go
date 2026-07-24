//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	catalogdomain "github.com/forecastiq/forecastiq/internal/catalog/domain"
)

// TestAPI_AdminHealthRequiresAdmin gates the endpoint behind the admin role.
func TestAPI_AdminHealthRequiresAdmin(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	assert.Equal(t, http.StatusUnauthorized,
		doRequest(e, http.MethodGet, "/api/v1/admin/health", "", nil).Code)
}

// TestAPI_AdminHealth assembles the S-10 view after a collection + observation:
// a cell with last_success + freshness, a provider circuit, the observation
// collector, and the system section — all from application tables.
func TestAPI_AdminHealth(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(3))
	e.seedCatalog(ctx, t)

	// A successful collection creates a forecast_collections row + a circuit.
	rec := doRequest(e, http.MethodPost, "/api/v1/admin/collections/trigger", adminToken, map[string]any{
		"provider_id": catalogdomain.OpenMeteoProviderID.String(),
		"location_id": catalogdomain.JohorBahruLocationID.String(),
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	insertContextObservation(ctx, t, e.pool, 30.0, 0.0, time.Now().UTC().Add(-30*time.Minute))

	rec = doRequest(e, http.MethodGet, "/api/v1/admin/health", adminToken, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))

	env := decodeEnvelope(t, rec)
	data := env["data"].(map[string]any)

	cells := data["cells"].([]any)
	require.GreaterOrEqual(t, len(cells), 1)
	cell := cells[0].(map[string]any)
	assert.Equal(t, "Open-Meteo", cell["provider"].(map[string]any)["name"])
	assert.NotNil(t, cell["last_success_at"])
	assert.NotNil(t, cell["freshness"])

	circuits := data["circuits"].([]any)
	assert.GreaterOrEqual(t, len(circuits), 1)

	obs := data["observation_collector"].(map[string]any)
	assert.EqualValues(t, 1, obs["locations_covered"])
	assert.GreaterOrEqual(t, len(obs["locations"].([]any)), 1)

	// system section present (payload volume from statfs is omitted in tests —
	// the test env wires a nil stater — but the section object exists).
	_, hasSystem := data["system"]
	assert.True(t, hasSystem)
}

// TestAPI_AdminHealthStatusFilter filters cells by freshness state.
func TestAPI_AdminHealthStatusFilter(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)
	doRequest(e, http.MethodPost, "/api/v1/admin/collections/trigger", adminToken, map[string]any{
		"provider_id": catalogdomain.OpenMeteoProviderID.String(),
		"location_id": catalogdomain.JohorBahruLocationID.String(),
	})

	// The just-collected cell is fresh; filtering to stale yields no cells.
	rec := doRequest(e, http.MethodGet, "/api/v1/admin/health?status=stale", adminToken, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	cells := decodeEnvelope(t, rec)["data"].(map[string]any)["cells"].([]any)
	assert.Empty(t, cells)

	rec = doRequest(e, http.MethodGet, "/api/v1/admin/health?status=fresh", adminToken, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	cells = decodeEnvelope(t, rec)["data"].(map[string]any)["cells"].([]any)
	assert.GreaterOrEqual(t, len(cells), 1)
}
