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

// doRequestAPIKey issues a request authenticated by an X-API-Key header (the
// programmatic auth path), as opposed to the Bearer path used by doRequest.
func doRequestAPIKey(e *testEnv, method, path, apiKey string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-API-Key", apiKey)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// TestAuthzMatrix_PublicEndpoints: the AUTH-08 public set is reachable with no
// token (no 401/403). Derived data with attribution — portfolio visibility.
func TestAuthzMatrix_PublicEndpoints(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	for _, path := range []string{
		"/api/v1/locations",
		"/api/v1/providers",
		"/api/v1/rankings/methodology",
	} {
		rec := doRequest(e, http.MethodGet, path, "", nil)
		assert.Equal(t, http.StatusOK, rec.Code, "public GET %s", path)
	}
}

// TestAuthzMatrix_AdminGating: admin endpoints require RequireAuth +
// RequireRole("admin") — no token 401, a user-role principal 403, admin 200.
func TestAuthzMatrix_AdminGating(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	// No token → 401.
	assert.Equal(t, http.StatusUnauthorized,
		doRequest(e, http.MethodGet, "/api/v1/admin/audit-events", "", nil).Code)

	// User role (provisioned on first auth) → 403.
	assert.Equal(t, http.StatusForbidden,
		doRequest(e, http.MethodGet, "/api/v1/admin/audit-events", "alice", nil).Code)

	// Admin role → 200.
	assert.Equal(t, http.StatusOK,
		doRequest(e, http.MethodGet, "/api/v1/admin/audit-events", adminToken, nil).Code)
}

// TestAuthzMatrix_SelfService: /me + /api-keys are any-authenticated (S-09);
// unauthenticated is 401; a user sees + edits its own profile.
func TestAuthzMatrix_SelfService(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	// Unauthenticated → 401.
	assert.Equal(t, http.StatusUnauthorized, doRequest(e, http.MethodGet, "/api/v1/me", "", nil).Code)

	// User provisions + reads own profile (role=user, not admin).
	rec := doRequest(e, http.MethodGet, "/api/v1/me", "alice:alice@example.com", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	user := decodeEnvelope(t, rec)["data"].(map[string]any)["user"].(map[string]any)
	assert.Equal(t, "user", user["role"])
	assert.Equal(t, "alice@example.com", user["email"])
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))

	// User updates own preferences.
	rec = doRequest(e, http.MethodPatch, "/api/v1/me", "alice", map[string]any{
		"preferences": map[string]any{"theme": "dark"},
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	updated := decodeEnvelope(t, rec)["data"].(map[string]any)["user"].(map[string]any)
	assert.Equal(t, "dark", updated["preferences"].(map[string]any)["theme"])
}

// TestAuthzMatrix_APIKeyLifecycleAndOwnership: create (plaintext once) → list
// (no secret) → object-level ownership on revoke (non-owner 404, no existence
// disclosure) → owner revoke 204.
func TestAuthzMatrix_APIKeyLifecycleAndOwnership(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	// bob creates a key.
	rec := doRequest(e, http.MethodPost, "/api/v1/api-keys", "bob", map[string]any{
		"name": "ci-key", "scopes": []string{"read:data"},
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	data := decodeEnvelope(t, rec)["data"].(map[string]any)
	plaintext := data["key"].(string)
	keyID := data["api_key"].(map[string]any)["id"].(string)
	assert.NotEmpty(t, plaintext)
	assert.NotContains(t, rec.Body.String(), "key_hash")

	// bob lists keys (metadata only; the secret is never returned again).
	rec = doRequest(e, http.MethodGet, "/api/v1/api-keys", "bob", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	keys := decodeEnvelope(t, rec)["data"].(map[string]any)["api_keys"].([]any)
	assert.Len(t, keys, 1)
	assert.NotContains(t, rec.Body.String(), plaintext)

	// carol (a different user) cannot revoke bob's key → 404 (no existence leak).
	assert.Equal(t, http.StatusNotFound,
		doRequest(e, http.MethodDelete, "/api/v1/api-keys/"+keyID, "carol", nil).Code)

	// bob revokes own key → 204.
	assert.Equal(t, http.StatusNoContent,
		doRequest(e, http.MethodDelete, "/api/v1/api-keys/"+keyID, "bob", nil).Code)
}

// TestAuthzMatrix_RawDataScope: /forecasts/latest is gated raw data. Public
// 401; a JWT session (full scope) 200; an API key needs the read:data scope
// (default read:public → 403); a revoked key → 401.
func TestAuthzMatrix_RawDataScope(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(3))
	e.seedCatalog(ctx, t)

	_, err := e.collector.Collect(ctx, collectInput(ctx, t, e))
	require.NoError(t, err)

	path := "/api/v1/forecasts/latest?provider_id=" + catalogdomain.OpenMeteoProviderID.String() +
		"&location_id=" + catalogdomain.JohorBahruLocationID.String()

	// Public → 401.
	assert.Equal(t, http.StatusUnauthorized, doRequest(e, http.MethodGet, path, "", nil).Code)

	// JWT session (full user rights) → 200.
	assert.Equal(t, http.StatusOK, doRequest(e, http.MethodGet, path, "dave", nil).Code)

	// API key without read:data (default read:public) → 403.
	recPub := doRequest(e, http.MethodPost, "/api/v1/api-keys", "dave", map[string]any{"name": "pub"})
	require.Equal(t, http.StatusCreated, recPub.Code, recPub.Body.String())
	pubKey := decodeEnvelope(t, recPub)["data"].(map[string]any)["key"].(string)
	assert.Equal(t, http.StatusForbidden, doRequestAPIKey(e, http.MethodGet, path, pubKey, nil).Code)

	// API key with read:data → 200.
	recData := doRequest(e, http.MethodPost, "/api/v1/api-keys", "dave", map[string]any{
		"name": "data", "scopes": []string{"read:data"},
	})
	require.Equal(t, http.StatusCreated, recData.Code)
	dataEnv := decodeEnvelope(t, recData)["data"].(map[string]any)
	dataKey := dataEnv["key"].(string)
	dataKeyID := dataEnv["api_key"].(map[string]any)["id"].(string)
	assert.Equal(t, http.StatusOK, doRequestAPIKey(e, http.MethodGet, path, dataKey, nil).Code)

	// Revoke the read:data key → it no longer authenticates (401).
	require.Equal(t, http.StatusNoContent,
		doRequest(e, http.MethodDelete, "/api/v1/api-keys/"+dataKeyID, "dave", nil).Code)
	assert.Equal(t, http.StatusUnauthorized, doRequestAPIKey(e, http.MethodGet, path, dataKey, nil).Code)
}

// TestAuthzMatrix_DisabledUserImmediate: the role/status come from the database
// per request (ADR-017), so disabling a user takes effect on the next call.
func TestAuthzMatrix_DisabledUserImmediate(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	// Provision + authenticate the user.
	require.Equal(t, http.StatusOK, doRequest(e, http.MethodGet, "/api/v1/me", "erin", nil).Code)

	// Admin disables the account directly (Supabase propagation is WP-19b).
	_, err := e.pool.Exec(ctx, `UPDATE users SET status = 'disabled' WHERE auth_subject = $1`, "dev|erin")
	require.NoError(t, err)

	// The same token is now rejected (401) — no token-lifetime lag.
	assert.Equal(t, http.StatusUnauthorized, doRequest(e, http.MethodGet, "/api/v1/me", "erin", nil).Code)
}
