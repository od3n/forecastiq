//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	catalogdomain "github.com/forecastiq/forecastiq/internal/catalog/domain"
)

// requestExport POSTs /me/export and returns the completed job's id.
func requestMyExport(t *testing.T, e *testEnv, token string) string {
	t.Helper()
	rec := doRequest(e, http.MethodPost, "/api/v1/me/export", token, nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	exp := decodeEnvelope(t, rec)["data"].(map[string]any)["export"].(map[string]any)
	assert.Equal(t, "completed", exp["status"])
	return exp["id"].(string)
}

// TestExport_SelfRequestAndDownload: a user requests their own export and
// downloads the account-data JSON (user + keys metadata + own audit events).
func TestExport_SelfRequestAndDownload(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	// jane provisions + owns a key (so the export has key + audit content).
	require.Equal(t, http.StatusCreated,
		doRequest(e, http.MethodPost, "/api/v1/api-keys", "jane", map[string]any{"name": "k"}).Code)

	id := requestMyExport(t, e, "jane")
	dl := doRequest(e, http.MethodGet, "/api/v1/exports/"+id, "jane", nil)
	require.Equal(t, http.StatusOK, dl.Code, dl.Body.String())
	assert.Equal(t, "no-store", dl.Header().Get("Cache-Control"))

	var doc map[string]any
	require.NoError(t, json.Unmarshal(dl.Body.Bytes(), &doc))
	assert.Equal(t, "jane@dev.local", doc["user"].(map[string]any)["email"])
	assert.GreaterOrEqual(t, len(doc["api_keys"].([]any)), 1)
	assert.GreaterOrEqual(t, len(doc["audit_events"].([]any)), 1)
	// The secret hash is never present in an export.
	assert.NotContains(t, dl.Body.String(), "key_hash")
}

// TestExport_OneActivePerUser: a pending job blocks a new request (409, D-06).
func TestExport_OneActivePerUser(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	janeID := meID(t, e, "jane")
	_, err := e.pool.Exec(ctx,
		`INSERT INTO export_jobs (id, workspace_id, requested_by, target_user_id, status, created_at)
		 VALUES ($1, $2, $3, $3, 'pending', now())`,
		uuid.New(), catalogdomain.SystemWorkspaceID, uuid.MustParse(janeID))
	require.NoError(t, err)

	assert.Equal(t, http.StatusConflict,
		doRequest(e, http.MethodPost, "/api/v1/me/export", "jane", nil).Code)
}

// TestExport_DownloadAuthorization: only the requester, the target, or an admin
// may download; a non-owner gets 404 (no existence disclosure).
func TestExport_DownloadAuthorization(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	id := requestMyExport(t, e, "jane")

	// A different, non-admin user cannot see it.
	assert.Equal(t, http.StatusNotFound,
		doRequest(e, http.MethodGet, "/api/v1/exports/"+id, "ken", nil).Code)
	// The owner and an admin can.
	assert.Equal(t, http.StatusOK, doRequest(e, http.MethodGet, "/api/v1/exports/"+id, "jane", nil).Code)
	assert.Equal(t, http.StatusOK, doRequest(e, http.MethodGet, "/api/v1/exports/"+id, adminToken, nil).Code)
	// Unknown id → 404.
	assert.Equal(t, http.StatusNotFound,
		doRequest(e, http.MethodGet, "/api/v1/exports/"+uuid.NewString(), "jane", nil).Code)
}

// TestExport_AdminTriggered: an admin exports a target user's data; a non-admin
// cannot use the admin endpoint (403). The admin may download the result.
func TestExport_AdminTriggered(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	leoID := meID(t, e, "leo")

	// Non-admin is refused by the role gate.
	assert.Equal(t, http.StatusForbidden,
		doRequest(e, http.MethodPost, "/api/v1/admin/users/"+leoID+"/export", "mia", nil).Code)

	// Admin triggers the export for leo.
	rec := doRequest(e, http.MethodPost, "/api/v1/admin/users/"+leoID+"/export", adminToken, nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	id := decodeEnvelope(t, rec)["data"].(map[string]any)["export"].(map[string]any)["id"].(string)

	dl := doRequest(e, http.MethodGet, "/api/v1/exports/"+id, adminToken, nil)
	require.Equal(t, http.StatusOK, dl.Code, dl.Body.String())
	var doc map[string]any
	require.NoError(t, json.Unmarshal(dl.Body.Bytes(), &doc))
	assert.Equal(t, "leo@dev.local", doc["user"].(map[string]any)["email"])
}

// TestExport_ExpiredDownload: a completed export past its 24h window is 410.
func TestExport_ExpiredDownload(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	id := requestMyExport(t, e, "nora")
	_, err := e.pool.Exec(ctx,
		`UPDATE export_jobs SET expires_at = now() - interval '1 hour' WHERE id = $1`, uuid.MustParse(id))
	require.NoError(t, err)

	assert.Equal(t, http.StatusGone, doRequest(e, http.MethodGet, "/api/v1/exports/"+id, "nora", nil).Code)
}

// TestExport_DeleteRequesterCascades: an account with a prior export stays
// deletable (requested_by CASCADE), and the export row is removed with it.
func TestExport_DeleteRequesterCascades(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	opalID := meID(t, e, "opal")
	_ = requestMyExport(t, e, "opal")

	// The user (having requested an export) is still deletable.
	require.Equal(t, http.StatusNoContent,
		doRequest(e, http.MethodDelete, "/api/v1/admin/users/"+opalID, adminToken, nil).Code)

	var jobs int
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT count(*) FROM export_jobs WHERE requested_by = $1`, uuid.MustParse(opalID)).Scan(&jobs))
	assert.Equal(t, 0, jobs)
}
