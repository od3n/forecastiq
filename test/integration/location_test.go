//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	catalogdomain "github.com/forecastiq/forecastiq/internal/catalog/domain"
)

// ── BR-LOC-01: dedup 409 + override ──────────────────────────────────

func TestAPI_DuplicateLocation409(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	// 0.01° from the seeded Johor Bahru location — inside the 0.05° boundary.
	rec := doRequest(e, http.MethodPost, "/api/v1/locations", adminToken, map[string]any{
		"name": "JB Copy", "latitude": 1.5027, "longitude": 103.7414,
		"country_code": "MY", "timezone": "Asia/Kuala_Lumpur",
	})
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())

	var problem map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &problem))
	assert.Contains(t, problem["type"], "duplicate")
	assert.Equal(t, "Duplicate Location", problem["title"])
	assert.NotEmpty(t, problem["request_id"])

	existing := problem["existing_resource"].(map[string]any)
	assert.Equal(t, catalogdomain.JohorBahruLocationID.String(), existing["id"])
	assert.Equal(t, "Johor Bahru", existing["name"])
	dist := existing["distance_degrees"].(float64)
	assert.Greater(t, dist, 0.0)
	assert.Less(t, dist, 0.05)
}

func TestAPI_DuplicateOverrideWithReason(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	rec := doRequest(e, http.MethodPost, "/api/v1/locations", adminToken, map[string]any{
		"name": "JB Harbour", "latitude": 1.5027, "longitude": 103.7414,
		"country_code": "MY", "timezone": "Asia/Kuala_Lumpur",
		"allow_near_duplicate": true, "override_reason": "distinct harbour monitoring site",
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	// Audit row records the override flag and reason.
	var details map[string]any
	err := e.pool.QueryRow(ctx,
		`SELECT details FROM audit_events WHERE action = 'location.create'
		 AND details->>'name' = 'JB Harbour'`).Scan(&details)
	require.NoError(t, err)
	assert.Equal(t, "true", details["allow_near_duplicate"])
	assert.Equal(t, "distinct harbour monitoring site", details["override_reason"])
}

func TestAPI_LocationNameConflict409(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	// Far from JB (no proximity dedup) but the same name as an active location.
	rec := doRequest(e, http.MethodPost, "/api/v1/locations", adminToken, map[string]any{
		"name": "Johor Bahru", "latitude": 50.0, "longitude": 10.0,
		"country_code": "DE", "timezone": "Europe/Berlin",
	})
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	var problem map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &problem))
	assert.Contains(t, problem["type"], "conflict")
}

// ── Update (PUT) ─────────────────────────────────────────────────────

func TestAPI_UpdateLocation(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	id := catalogdomain.JohorBahruLocationID.String()
	rec := doRequest(e, http.MethodPut, "/api/v1/locations/"+id, adminToken, map[string]any{
		"name": "Johor Bahru City",
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	env := decodeEnvelope(t, rec)
	data := env["data"].(map[string]any)
	assert.Equal(t, "Johor Bahru City", data["name"])
	// Immutable fields unchanged.
	assert.Equal(t, "Asia/Kuala_Lumpur", data["timezone"])

	// Persisted.
	rec = doRequest(e, http.MethodGet, "/api/v1/locations/"+id, "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	env = decodeEnvelope(t, rec)
	assert.Equal(t, "Johor Bahru City", env["data"].(map[string]any)["name"])

	// Audited.
	var details map[string]any
	err := e.pool.QueryRow(ctx,
		`SELECT details FROM audit_events WHERE action = 'location.update'`).Scan(&details)
	require.NoError(t, err)
	changes := details["changes"].(map[string]any)
	assert.Equal(t, "Johor Bahru City", changes["name"])
}

func TestAPI_UpdateLocationRequiresAuth(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	rec := doRequest(e, http.MethodPut, "/api/v1/locations/"+catalogdomain.JohorBahruLocationID.String(),
		"", map[string]any{"name": "X"})
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAPI_UpdateLocationNotFound(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	rec := doRequest(e, http.MethodPut, "/api/v1/locations/"+mustUUIDv7().String(), adminToken,
		map[string]any{"name": "X"})
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAPI_UpdateLocationInvalidName(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	rec := doRequest(e, http.MethodPut, "/api/v1/locations/"+catalogdomain.JohorBahruLocationID.String(),
		adminToken, map[string]any{"name": ""})
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
}

// ── Status lifecycle (PATCH) ─────────────────────────────────────────

func TestAPI_SetLocationStatus(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	id := catalogdomain.JohorBahruLocationID.String()

	// Disable.
	rec := doRequest(e, http.MethodPatch, "/api/v1/locations/"+id+"/status", adminToken,
		map[string]any{"status": "disabled"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	env := decodeEnvelope(t, rec)
	assert.Equal(t, "disabled", env["data"].(map[string]any)["status"])

	// Audited.
	var details map[string]any
	err := e.pool.QueryRow(ctx,
		`SELECT details FROM audit_events WHERE action = 'location.set_status'`).Scan(&details)
	require.NoError(t, err)
	assert.Equal(t, "disabled", details["status"])

	// Re-enable.
	rec = doRequest(e, http.MethodPatch, "/api/v1/locations/"+id+"/status", adminToken,
		map[string]any{"status": "active"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	env = decodeEnvelope(t, rec)
	assert.Equal(t, "active", env["data"].(map[string]any)["status"])
}

func TestAPI_SetLocationStatusInvalid(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	rec := doRequest(e, http.MethodPatch,
		"/api/v1/locations/"+catalogdomain.JohorBahruLocationID.String()+"/status", adminToken,
		map[string]any{"status": "bogus"})
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestAPI_SetLocationStatusRequiresAuth(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	rec := doRequest(e, http.MethodPatch,
		"/api/v1/locations/"+catalogdomain.JohorBahruLocationID.String()+"/status", "",
		map[string]any{"status": "disabled"})
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// ── BR-LOC-03: disable stops future collection, history remains ──────

func TestAPI_DisabledLocationBlocksCollection(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(3))
	e.seedCatalog(ctx, t)

	id := catalogdomain.JohorBahruLocationID.String()

	// Collect once while active — historical data exists.
	rec := doRequest(e, http.MethodPost, "/api/v1/admin/collections/trigger", adminToken, map[string]any{
		"provider_id": catalogdomain.OpenMeteoProviderID.String(), "location_id": id,
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Disable the location.
	rec = doRequest(e, http.MethodPatch, "/api/v1/locations/"+id+"/status", adminToken,
		map[string]any{"status": "disabled"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Trigger now fails 422 (resource inactive).
	rec = doRequest(e, http.MethodPost, "/api/v1/admin/collections/trigger", adminToken, map[string]any{
		"provider_id": catalogdomain.OpenMeteoProviderID.String(), "location_id": id,
	})
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())

	// Excluded from the active list.
	rec = doRequest(e, http.MethodGet, "/api/v1/locations?active=true", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	env := decodeEnvelope(t, rec)
	locations := env["data"].(map[string]any)["locations"].([]any)
	assert.Empty(t, locations)

	// Historical data remains queryable (BR-LOC-03).
	rec = doRequest(e, http.MethodGet,
		"/api/v1/forecasts/latest?provider_id="+catalogdomain.OpenMeteoProviderID.String()+
			"&location_id="+id, "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	env = decodeEnvelope(t, rec)
	snapshots := env["data"].(map[string]any)["snapshots"].([]any)
	assert.Len(t, snapshots, 3)
}

// ── Scheduler eligibility (service level) ────────────────────────────

func TestSchedulerEligibility_DisabledLocationExcluded(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	active, err := e.locations.ListActiveLocations(ctx)
	require.NoError(t, err)
	require.Len(t, active, 1)

	// Disable through the public API.
	rec := doRequest(e, http.MethodPatch,
		"/api/v1/locations/"+catalogdomain.JohorBahruLocationID.String()+"/status", adminToken,
		map[string]any{"status": "disabled"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	active, err = e.locations.ListActiveLocations(ctx)
	require.NoError(t, err)
	assert.Empty(t, active, "disabled locations must not generate collection slots")
}

// ── DRB-WP04-001: concurrent dedup safety ─────────────────────────────

// TestAPI_ConcurrentDuplicateCreates verifies that N concurrent near-duplicate
// creates at the same coordinates produce exactly one 201 and N-1 409s, with
// exactly one row persisted (BR-LOC-01 under concurrency).
func TestAPI_ConcurrentDuplicateCreates(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	const n = 6
	var wg sync.WaitGroup
	codes := make([]int, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			rec := doRequest(e, http.MethodPost, "/api/v1/locations", adminToken, map[string]any{
				"name":     "Storm Loc " + string(rune('A'+idx)),
				"latitude": 30.001, "longitude": 60.001,
				"country_code": "IR", "timezone": "Asia/Tehran",
			})
			codes[idx] = rec.Code
		}(i)
	}
	wg.Wait()

	created := 0
	conflict := 0
	for _, code := range codes {
		switch code {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflict++
		}
	}
	assert.Equal(t, 1, created, "exactly one create must succeed")
	assert.Equal(t, n-1, conflict, "all others must be 409")

	// Exactly one row persisted at those coordinates.
	var count int
	err := e.pool.QueryRow(ctx,
		`SELECT count(*) FROM locations WHERE latitude = 30.001 AND longitude = 60.001`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "exactly one row must be persisted")
}

// ── DRB-WP04-003: override reason mandatory ───────────────────────────

func TestAPI_OverrideWithoutReason422(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	// allow_near_duplicate without override_reason → 422.
	rec := doRequest(e, http.MethodPost, "/api/v1/locations", adminToken, map[string]any{
		"name": "JB Copy", "latitude": 1.5027, "longitude": 103.7414,
		"country_code": "MY", "timezone": "Asia/Kuala_Lumpur",
		"allow_near_duplicate": true,
	})
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())

	var problem map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &problem))
	errs := problem["errors"].([]any)
	assert.Contains(t, errs[0].(map[string]any)["field"], "override_reason")
}

// ── DRB-WP04-004: status lifecycle restrictions ───────────────────────

func TestAPI_SetLocationStatusArchivedRejected(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	rec := doRequest(e, http.MethodPatch,
		"/api/v1/locations/"+catalogdomain.JohorBahruLocationID.String()+"/status", adminToken,
		map[string]any{"status": "archived"})
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
}

func TestAPI_SetLocationStatusNoOpRejected(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	id := catalogdomain.JohorBahruLocationID.String()

	// active → active is a no-op → 409.
	rec := doRequest(e, http.MethodPatch, "/api/v1/locations/"+id+"/status", adminToken,
		map[string]any{"status": "active"})
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())

	// Disable, then disabled → disabled is a no-op → 409.
	rec = doRequest(e, http.MethodPatch, "/api/v1/locations/"+id+"/status", adminToken,
		map[string]any{"status": "disabled"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = doRequest(e, http.MethodPatch, "/api/v1/locations/"+id+"/status", adminToken,
		map[string]any{"status": "disabled"})
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
}
