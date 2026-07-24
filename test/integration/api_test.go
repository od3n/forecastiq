//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	catalogdomain "github.com/forecastiq/forecastiq/internal/catalog/domain"
)

const adminToken = "test-admin-token"

func doRequest(e *testEnv, method, path, token string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var env map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	return env
}

func TestAPI_HealthProbes(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))

	assert.Equal(t, http.StatusOK, doRequest(e, http.MethodGet, "/healthz", "", nil).Code)
	assert.Equal(t, http.StatusOK, doRequest(e, http.MethodGet, "/readyz", "", nil).Code)
}

func TestAPI_CreateLocationRequiresAuth(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	rec := doRequest(e, http.MethodPost, "/api/v1/locations", "", map[string]any{
		"name": "X", "latitude": 1.0, "longitude": 2.0, "country_code": "MY", "timezone": "Asia/Kuala_Lumpur",
	})
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAPI_CreateAndListLocation(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	// Create a new location (admin).
	rec := doRequest(e, http.MethodPost, "/api/v1/locations", adminToken, map[string]any{
		"name": "Kuala Lumpur", "latitude": 3.1390, "longitude": 101.6869,
		"country_code": "MY", "timezone": "Asia/Kuala_Lumpur",
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	env := decodeEnvelope(t, rec)
	data := env["data"].(map[string]any)
	assert.Equal(t, "Kuala Lumpur", data["name"])

	// List includes the seeded JB location + the new one.
	rec = doRequest(e, http.MethodGet, "/api/v1/locations", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	env = decodeEnvelope(t, rec)
	locations := env["data"].(map[string]any)["locations"].([]any)
	assert.GreaterOrEqual(t, len(locations), 2)
}

func TestAPI_TriggerCollectionAndLatestForecast(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(3))
	e.seedCatalog(ctx, t)

	// Trigger a manual collection.
	rec := doRequest(e, http.MethodPost, "/api/v1/admin/collections/trigger", adminToken, map[string]any{
		"provider_id": catalogdomain.OpenMeteoProviderID.String(),
		"location_id": catalogdomain.JohorBahruLocationID.String(),
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	env := decodeEnvelope(t, rec)
	collData := env["data"].(map[string]any)
	assert.Equal(t, "success", collData["status"])
	assert.EqualValues(t, 3, collData["snapshots_stored"])

	// Latest forecast returns the collection + snapshots + attribution.
	// Raw forecast data is gated (RequireAuth + read:data scope); use the admin
	// principal (JWT session ⇒ full scope).
	rec = doRequest(e, http.MethodGet,
		"/api/v1/forecasts/latest?provider_id="+catalogdomain.OpenMeteoProviderID.String()+
			"&location_id="+catalogdomain.JohorBahruLocationID.String(), adminToken, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	env = decodeEnvelope(t, rec)
	data := env["data"].(map[string]any)
	snapshots := data["snapshots"].([]any)
	assert.Len(t, snapshots, 3)
	attribution := env["attribution"].([]any)
	assert.NotEmpty(t, attribution)
	assert.NotNil(t, env["freshness"])

	// Collection lineage query (admin).
	rec = doRequest(e, http.MethodGet, "/api/v1/forecast-collections", adminToken, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	env = decodeEnvelope(t, rec)
	collections := env["data"].(map[string]any)["collections"].([]any)
	assert.GreaterOrEqual(t, len(collections), 1)
}

func TestAPI_ListCollectionsRequiresAuth(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	rec := doRequest(e, http.MethodGet, "/api/v1/forecast-collections", "", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAPI_OpenAPIServed(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))

	rec := doRequest(e, http.MethodGet, "/api/v1/openapi.json", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	var spec map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &spec))
	assert.Equal(t, "3.1.0", spec["openapi"])
}

func TestAPI_ValidationErrorShape(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	// Invalid latitude → 422 with RFC 7807 problem + errors[].
	rec := doRequest(e, http.MethodPost, "/api/v1/locations", adminToken, map[string]any{
		"name": "Bad", "latitude": 999.0, "longitude": 101.0, "country_code": "MY", "timezone": "Asia/Kuala_Lumpur",
	})
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	var problem map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &problem))
	assert.Contains(t, problem["type"], "validation")
	assert.NotEmpty(t, problem["request_id"])
	assert.NotNil(t, problem["errors"])
}
