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

// TestAPI_SetProviderStatus enables/disables a provider (admin) and rejects
// unauthenticated calls + reserved statuses.
func TestAPI_SetProviderStatus(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)
	pid := catalogdomain.OpenMeteoProviderID.String()

	assert.Equal(t, http.StatusUnauthorized,
		doRequest(e, http.MethodPatch, "/api/v1/admin/providers/"+pid+"/status", "", map[string]any{"status": "disabled"}).Code)

	rec := doRequest(e, http.MethodPatch, "/api/v1/admin/providers/"+pid+"/status", adminToken, map[string]any{"status": "disabled"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "disabled", decodeEnvelope(t, rec)["data"].(map[string]any)["status"])

	rec = doRequest(e, http.MethodPatch, "/api/v1/admin/providers/"+pid+"/status", adminToken, map[string]any{"status": "active"})
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "active", decodeEnvelope(t, rec)["data"].(map[string]any)["status"])

	// archived is reserved → 422.
	assert.Equal(t, http.StatusUnprocessableEntity,
		doRequest(e, http.MethodPatch, "/api/v1/admin/providers/"+pid+"/status", adminToken, map[string]any{"status": "archived"}).Code)
}

// TestAPI_UpdateProviderConfiguration edits operator-mutable fields and never
// echoes the credential reference.
func TestAPI_UpdateProviderConfiguration(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)
	cid := catalogdomain.OpenMeteoConfigID.String()

	rec := doRequest(e, http.MethodPatch, "/api/v1/admin/provider-configurations/"+cid, adminToken, map[string]any{
		"status": "disabled", "minute_offset": 7, "validation_state": "validated",
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	data := decodeEnvelope(t, rec)["data"].(map[string]any)
	assert.Equal(t, "disabled", data["status"])
	assert.EqualValues(t, 7, data["collection_schedule"].(map[string]any)["minute_offset"])
	assert.Equal(t, "validated", data["validation_state"])
	// credential is never echoed: has_credential boolean present, no ref field.
	_, hasRef := data["credential_ref"]
	assert.False(t, hasRef)
	_, hasFlag := data["has_credential"]
	assert.True(t, hasFlag)

	// Out-of-range minute offset → 422.
	assert.Equal(t, http.StatusUnprocessableEntity,
		doRequest(e, http.MethodPatch, "/api/v1/admin/provider-configurations/"+cid, adminToken, map[string]any{"minute_offset": 60}).Code)
}
