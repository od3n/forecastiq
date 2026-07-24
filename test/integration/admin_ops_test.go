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

// TestAPI_AuditEvents lists the audit trail (admin) and reflects a mutation.
func TestAPI_AuditEvents(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	assert.Equal(t, http.StatusUnauthorized,
		doRequest(e, http.MethodGet, "/api/v1/admin/audit-events", "", nil).Code)

	// Perform an audited mutation, then confirm it appears in the trail.
	doRequest(e, http.MethodPatch, "/api/v1/admin/providers/"+catalogdomain.OpenMeteoProviderID.String()+"/status",
		adminToken, map[string]any{"status": "disabled"})

	rec := doRequest(e, http.MethodGet, "/api/v1/admin/audit-events?action=provider.set_status", adminToken, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	events := decodeEnvelope(t, rec)["data"].(map[string]any)["audit_events"].([]any)
	require.GreaterOrEqual(t, len(events), 1)
	ev := events[0].(map[string]any)
	assert.Equal(t, "provider.set_status", ev["action"])
	assert.Equal(t, "provider", ev["resource_type"])
}

// TestAPI_AdminRecompute runs the analysis pipeline on demand and audits it.
func TestAPI_AdminRecompute(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	assert.Equal(t, http.StatusUnauthorized,
		doRequest(e, http.MethodPost, "/api/v1/admin/recompute", "", nil).Code)

	rec := doRequest(e, http.MethodPost, "/api/v1/admin/recompute", adminToken, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	data := decodeEnvelope(t, rec)["data"].(map[string]any)
	// No matched data yet → 0 records affected, but the pipeline runs cleanly.
	assert.EqualValues(t, 0, data["records_affected"])

	// The trigger is audited.
	rec = doRequest(e, http.MethodGet, "/api/v1/admin/audit-events?action=analysis.recompute", adminToken, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	events := decodeEnvelope(t, rec)["data"].(map[string]any)["audit_events"].([]any)
	require.Len(t, events, 1)
	assert.Equal(t, "analysis", events[0].(map[string]any)["resource_type"])
}
