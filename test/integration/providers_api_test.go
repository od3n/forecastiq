//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	catalogdomain "github.com/forecastiq/forecastiq/internal/catalog/domain"
)

// TestAPI_ProvidersLineage exposes adapter_version + collecting_since once a
// provider has a successful collection (§4.1), on both list and detail.
func TestAPI_ProvidersLineage(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(3))
	e.seedCatalog(ctx, t)

	// Before any collection: provider present, lineage fields absent.
	rec := doRequest(e, http.MethodGet, "/api/v1/providers", "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	env := decodeEnvelope(t, rec)
	providers := env["data"].(map[string]any)["providers"].([]any)
	require.GreaterOrEqual(t, len(providers), 1)
	assert.NotEmpty(t, env["attribution"])
	first := providers[0].(map[string]any)
	_, hasVersion := first["adapter_version"]
	assert.False(t, hasVersion, "no adapter_version before any collection")

	// After a successful collection, lineage appears.
	doRequest(e, http.MethodPost, "/api/v1/admin/collections/trigger", adminToken, map[string]any{
		"provider_id": catalogdomain.OpenMeteoProviderID.String(),
		"location_id": catalogdomain.JohorBahruLocationID.String(),
	})

	rec = doRequest(e, http.MethodGet, "/api/v1/providers/"+catalogdomain.OpenMeteoProviderID.String(), "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	data := decodeEnvelope(t, rec)["data"].(map[string]any)
	assert.Equal(t, "Open-Meteo", data["name"])
	assert.Equal(t, "1.0.0-test", data["adapter_version"])
	assert.NotEmpty(t, data["collecting_since"])
}

// TestAPI_GetProviderNotFound returns 404 for an unknown provider id.
func TestAPI_GetProviderNotFound(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	rec := doRequest(e, http.MethodGet, "/api/v1/providers/"+mustUUIDv7().String(), "", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// TestAPI_LocationsBboxIgnored accepts the reserved bbox param without error.
func TestAPI_LocationsBboxIgnored(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	rec := doRequest(e, http.MethodGet, "/api/v1/locations?bbox=1,2,3,4", "", nil)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}
