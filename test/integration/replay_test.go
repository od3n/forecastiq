//go:build integration

package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	catalog "github.com/forecastiq/forecastiq/internal/catalog"
	catalogdomain "github.com/forecastiq/forecastiq/internal/catalog/domain"
	collectiondomain "github.com/forecastiq/forecastiq/internal/collection/domain"
	"github.com/forecastiq/forecastiq/internal/collection/ports"
)

// snapshotCount returns the total snapshots stored for the seeded location.
func snapshotCount(ctx context.Context, t *testing.T, e *testEnv) int {
	t.Helper()
	var n int
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT count(*) FROM forecast_snapshots WHERE location_id = $1`,
		catalogdomain.JohorBahruLocationID).Scan(&n))
	return n
}

// TestReplayIdempotency proves FC-14 replay creates a new collection from the
// stored payload without mutating the original, and that re-running is
// idempotent (no duplicate snapshots).
func TestReplayIdempotency(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(3))
	e.seedCatalog(ctx, t)

	orig, err := e.collector.Collect(ctx, collectInput(ctx, t, e))
	require.NoError(t, err)
	require.Equal(t, collectiondomain.StatusSuccess, orig.Status)
	require.Equal(t, 3, snapshotCount(ctx, t, e))

	// First replay: new collection, all snapshots already present → 0 stored.
	replayed, err := e.replayer.Replay(ctx, orig.ID, catalog.Actor{Name: "admin"})
	require.NoError(t, err)
	assert.NotEqual(t, orig.ID, replayed.ID, "replay creates a new collection")
	assert.Equal(t, 0, replayed.SnapshotsStored)
	assert.Equal(t, 3, replayed.SnapshotsDeduplicated)
	assert.Equal(t, 3, snapshotCount(ctx, t, e), "replay must not duplicate snapshots")

	// Second replay: still idempotent.
	again, err := e.replayer.Replay(ctx, orig.ID, catalog.Actor{Name: "admin"})
	require.NoError(t, err)
	assert.Equal(t, 0, again.SnapshotsStored)
	assert.Equal(t, 3, snapshotCount(ctx, t, e))

	// The original collection is untouched.
	reloaded, err := e.reader.GetCollection(ctx, orig.ID)
	require.NoError(t, err)
	assert.Equal(t, collectiondomain.StatusSuccess, reloaded.Status)
	assert.Equal(t, 3, reloaded.SnapshotsStored)
}

// TestReplayChecksumMismatchQuarantine proves a corrupt stored payload is
// detected (SHA-256 verify), quarantined, and surfaced as payload_unavailable.
func TestReplayChecksumMismatchQuarantine(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(3))
	e.seedCatalog(ctx, t)

	orig, err := e.collector.Collect(ctx, collectInput(ctx, t, e))
	require.NoError(t, err)
	require.NotEmpty(t, orig.RawPayloadObjectKey)

	// Corrupt the stored payload so its bytes no longer match the checksum.
	require.NoError(t, e.store.Write(ctx, orig.RawPayloadObjectKey, []byte(`{"tampered":true}`)))

	_, err = e.replayer.Replay(ctx, orig.ID, catalog.Actor{Name: "admin"})
	require.ErrorIs(t, err, collectiondomain.ErrPayloadUnavailable)

	// The corrupt payload was quarantined (renamed), so the original key is gone.
	_, readErr := e.store.Read(ctx, orig.RawPayloadObjectKey)
	assert.Error(t, readErr, "quarantined payload must no longer be readable at its key")
}

// TestReplayUnknownCollection returns not-found for an unknown id.
func TestReplayUnknownCollection(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	_, err := e.replayer.Replay(ctx, mustUUIDv7(), catalog.Actor{Name: "admin"})
	require.ErrorIs(t, err, collectiondomain.ErrNotFound)
}

// TestReplayDoesNotShadowLatestForecast is the DRB-WP08-001 regression guard:
// after replaying the most recent collection, /forecasts/latest must still
// return the original, complete snapshot set (the replay collection is recorded
// as deduplicated and must not become "latest successful").
func TestReplayDoesNotShadowLatestForecast(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(3))
	e.seedCatalog(ctx, t)

	orig, err := e.collector.Collect(ctx, collectInput(ctx, t, e))
	require.NoError(t, err)

	replayed, err := e.replayer.Replay(ctx, orig.ID, catalog.Actor{Name: "admin"})
	require.NoError(t, err)
	assert.Equal(t, collectiondomain.StatusDeduplicated, replayed.Status,
		"replay collection must be recorded as deduplicated, not success")

	// Latest forecast still resolves to the original complete collection.
	latest, err := e.reader.LatestForecast(ctx, catalogdomain.OpenMeteoProviderID, catalogdomain.JohorBahruLocationID)
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, orig.ID, latest.Collection.ID, "replay must not shadow the original as latest")
	assert.Len(t, latest.Snapshots, 3, "latest-forecast must retain the full snapshot set after replay")
}

// TestAPI_ReplayCollection exercises the admin replay endpoint end-to-end,
// including auth and not-found handling.
func TestAPI_ReplayCollection(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(3))
	e.seedCatalog(ctx, t)

	orig, err := e.collector.Collect(ctx, collectInput(ctx, t, e))
	require.NoError(t, err)

	// Unauthenticated → 401.
	rec := doRequest(e, http.MethodPost, "/api/v1/admin/collections/"+orig.ID.String()+"/replay", "", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// Admin → 200 with a new collection id.
	rec = doRequest(e, http.MethodPost, "/api/v1/admin/collections/"+orig.ID.String()+"/replay", adminToken, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	data := decodeEnvelope(t, rec)["data"].(map[string]any)
	assert.NotEqual(t, orig.ID.String(), data["id"])

	// Unknown id → 404.
	rec = doRequest(e, http.MethodPost, "/api/v1/admin/collections/"+mustUUIDv7().String()+"/replay", adminToken, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// TestAPI_TriggerRateLimited429 proves the manual-trigger budget guard: a
// rate-limited provider outcome is surfaced as 429 with Retry-After, not 200.
func TestAPI_TriggerRateLimited429(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, &fakeAdapter{count: 0, outcome: ports.OutcomeRateLimited})
	e.seedCatalog(ctx, t)

	rec := doRequest(e, http.MethodPost, "/api/v1/admin/collections/trigger", adminToken, map[string]any{
		"provider_id": catalogdomain.OpenMeteoProviderID.String(),
		"location_id": catalogdomain.JohorBahruLocationID.String(),
	})
	require.Equal(t, http.StatusTooManyRequests, rec.Code, rec.Body.String())
	assert.NotEmpty(t, rec.Header().Get("Retry-After"))
}
