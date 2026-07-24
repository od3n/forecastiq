//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// meID authenticates via GET /me and returns the principal's user id.
func meID(t *testing.T, e *testEnv, token string) string {
	t.Helper()
	rec := doRequest(e, http.MethodGet, "/api/v1/me", token, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeEnvelope(t, rec)["data"].(map[string]any)["user"].(map[string]any)["id"].(string)
}

// doWebhook posts a raw body to the auth-webhook receiver with the given
// signature header value (empty = omit the header).
func doWebhook(e *testEnv, body []byte, sig string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if sig != "" {
		req.Header.Set("X-Webhook-Signature", sig)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

func webhookSig(body []byte) string {
	mac := hmac.New(sha256.New, []byte(testWebhookSecret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// TestUserLifecycle_AdminListAndStatus: admin lists users, then disables a user
// (immediately effective — the disabled user can no longer authenticate) and
// re-enables them.
func TestUserLifecycle_AdminListAndStatus(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	frankID := meID(t, e, "frank")

	// Admin lists users (seeded admin + frank).
	rec := doRequest(e, http.MethodGet, "/api/v1/admin/users", adminToken, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	users := decodeEnvelope(t, rec)["data"].(map[string]any)["users"].([]any)
	assert.GreaterOrEqual(t, len(users), 2)

	// Disable frank → 200; frank can no longer authenticate.
	rec = doRequest(e, http.MethodPatch, "/api/v1/admin/users/"+frankID+"/status", adminToken,
		map[string]any{"status": "disabled"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, http.StatusUnauthorized, doRequest(e, http.MethodGet, "/api/v1/me", "frank", nil).Code)

	// Re-enable frank → 200; frank authenticates again.
	rec = doRequest(e, http.MethodPatch, "/api/v1/admin/users/"+frankID+"/status", adminToken,
		map[string]any{"status": "active"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, http.StatusOK, doRequest(e, http.MethodGet, "/api/v1/me", "frank", nil).Code)
}

// TestUserLifecycle_SelfLockoutGuard: an admin cannot disable or delete their
// own account through the admin surface (409).
func TestUserLifecycle_SelfLockoutGuard(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	adminID := meID(t, e, adminToken)

	assert.Equal(t, http.StatusConflict,
		doRequest(e, http.MethodPatch, "/api/v1/admin/users/"+adminID+"/status", adminToken,
			map[string]any{"status": "disabled"}).Code)
	assert.Equal(t, http.StatusConflict,
		doRequest(e, http.MethodDelete, "/api/v1/admin/users/"+adminID, adminToken, nil).Code)
}

// TestUserLifecycle_AdminDeleteAnonymizesAudit: deleting a user removes the row
// and cascades api_keys, while audit rows are preserved with user_id anonymized
// to NULL (audit-requirements §4).
func TestUserLifecycle_AdminDeleteAnonymizesAudit(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	graceID := meID(t, e, "grace")
	// grace owns an API key (must cascade on delete).
	require.Equal(t, http.StatusCreated,
		doRequest(e, http.MethodPost, "/api/v1/api-keys", "grace", map[string]any{"name": "k"}).Code)

	// Admin deletes grace.
	require.Equal(t, http.StatusNoContent,
		doRequest(e, http.MethodDelete, "/api/v1/admin/users/"+graceID, adminToken, nil).Code)

	// User row gone; api_keys cascaded.
	var userCount, keyCount int
	require.NoError(t, e.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE id = $1`, graceID).Scan(&userCount))
	assert.Equal(t, 0, userCount)
	require.NoError(t, e.pool.QueryRow(ctx, `SELECT count(*) FROM api_keys WHERE user_id = $1`, graceID).Scan(&keyCount))
	assert.Equal(t, 0, keyCount)

	// grace's provisioning audit row is preserved but anonymized (user_id NULL).
	var provRows, provNonNull int
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT count(*), count(user_id) FROM audit_events WHERE action = 'user.provisioned' AND resource_id = $1`,
		graceID).Scan(&provRows, &provNonNull))
	assert.Equal(t, 1, provRows, "audit row preserved")
	assert.Equal(t, 0, provNonNull, "actor anonymized (user_id NULL)")

	// The admin.user_deleted audit row survives with the admin actor intact.
	var delRows int
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT count(*) FROM audit_events WHERE action = 'admin.user_deleted' AND resource_id = $1 AND user_id IS NOT NULL`,
		graceID).Scan(&delRows))
	assert.Equal(t, 1, delRows)
}

// TestUserLifecycle_SelfDelete: DELETE /me removes the caller's own account.
func TestUserLifecycle_SelfDelete(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	henryID := meID(t, e, "henry")
	require.Equal(t, http.StatusNoContent, doRequest(e, http.MethodDelete, "/api/v1/me", "henry", nil).Code)

	var count int
	require.NoError(t, e.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE id = $1`, henryID).Scan(&count))
	assert.Equal(t, 0, count)

	// The self-delete audit row is preserved (anonymized).
	var delRows int
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT count(*) FROM audit_events WHERE action = 'user.deleted' AND resource_id = $1`,
		henryID).Scan(&delRows))
	assert.Equal(t, 1, delRows)
}

// TestAuthWebhook_SignatureAndAudit: a valid HMAC signature is accepted (204)
// and produces an auth.* audit row; a bad signature is rejected (401) with no
// audit row; an unrecognized event type is acknowledged without an audit row.
func TestAuthWebhook_SignatureAndAudit(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	body := []byte(`{"type":"login","subject":"dev|webhookuser"}`)

	// Bad signature → 401, no audit.
	assert.Equal(t, http.StatusUnauthorized, doWebhook(e, body, "sha256=deadbeef").Code)
	assert.Equal(t, http.StatusUnauthorized, doWebhook(e, body, "").Code)
	assert.Equal(t, 0, countAudit(ctx, t, e, "auth.login"))

	// Valid signature → 204 + audit row.
	require.Equal(t, http.StatusNoContent, doWebhook(e, body, webhookSig(body)).Code)
	assert.Equal(t, 1, countAudit(ctx, t, e, "auth.login"))

	// Unrecognized event type → 204, no audit row written.
	unknown := []byte(`{"type":"totally_unknown","subject":"x"}`)
	require.Equal(t, http.StatusNoContent, doWebhook(e, unknown, webhookSig(unknown)).Code)
	assert.Equal(t, 0, countAudit(ctx, t, e, "totally_unknown"))
}

func countAudit(ctx context.Context, t *testing.T, e *testEnv, action string) int {
	t.Helper()
	var n int
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT count(*) FROM audit_events WHERE action = $1`, action).Scan(&n))
	return n
}
