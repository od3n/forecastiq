//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/adapters/persistence/analysispg"
	"github.com/forecastiq/forecastiq/internal/analysis"
	catalogdomain "github.com/forecastiq/forecastiq/internal/catalog/domain"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
	"github.com/forecastiq/forecastiq/internal/platform/ids"
	"github.com/forecastiq/forecastiq/internal/platform/metrics"
)

// insertSnapshot inserts one forecast snapshot for the JB location, first
// creating the parent forecast_collections row it references (FK).
func insertSnapshot(ctx context.Context, t *testing.T, pool *pgxpool.Pool, id uuid.UUID, issuedAt, targetTime time.Time) {
	t.Helper()
	collID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO forecast_collections
		   (id, provider_id, location_id, provider_configuration_id, requested_at, collection_status)
		 VALUES ($1,$2,$3,$4,$5,'success')`,
		collID, catalogdomain.OpenMeteoProviderID, catalogdomain.JohorBahruLocationID,
		catalogdomain.OpenMeteoConfigID, issuedAt.UTC())
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO forecast_snapshots
		   (id, forecast_collection_id, provider_id, location_id, issued_at, target_time,
		    forecast_horizon_minutes, temperature_c, condition_taxonomy_version)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'1')`,
		id, collID, catalogdomain.OpenMeteoProviderID, catalogdomain.JohorBahruLocationID,
		issuedAt.UTC(), targetTime.UTC(), int(targetTime.Sub(issuedAt).Minutes()), 30.0)
	require.NoError(t, err)
}

// insertObservation inserts one observation row for the JB location.
func insertObservation(ctx context.Context, t *testing.T, pool *pgxpool.Pool, id uuid.UUID, observedAt time.Time, obsType, flag string, superseded *uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(ctx,
		`INSERT INTO observations
		   (id, location_id, source, observation_type, observed_at, temperature_c, quality_flag, superseded_observation_id)
		 VALUES ($1,$2,'openmeteo_historical',$3,$4,$5,$6,$7)`,
		id, catalogdomain.JohorBahruLocationID, obsType, observedAt.UTC(), 30.0, flag, superseded)
	require.NoError(t, err)
}

func newMatcher(pool *pgxpool.Pool) *analysis.MatchService {
	return analysis.NewMatchService(analysispg.NewMatchRepository(), dbtx.NewRunner(pool), pool,
		metrics.New(), clock.Real{}, slog_discard())
}

func pairCount(ctx context.Context, t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matched_evaluations`).Scan(&n))
	return n
}

// TestMatching_BatchDedupSuspect proves exact-hour matching, suspect exclusion
// (BR-MATCH-05: a snapshot whose only observation is suspect stays unmatched),
// and idempotent re-runs. The partial dedup index allows one live observation
// per (source, location, hour), so the suspect and valid rows are at different
// hours (distinct snapshots).
func TestMatching_BatchDedupSuspect(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	pool := newPool(ctx, t, connStr)
	(&testEnv{pool: pool}).seedCatalog(ctx, t)

	// Snapshot A at hour H1: only a SUSPECT observation → stays unmatched.
	h1 := time.Now().UTC().Truncate(time.Hour).Add(-5 * time.Hour)
	snapA := ids.New()
	insertSnapshot(ctx, t, pool, snapA, h1.Add(-time.Hour), h1)
	insertObservation(ctx, t, pool, ids.New(), h1, "reanalysis", "suspect", nil)

	// Snapshot B at hour H2: a VALID observation → matched.
	h2 := time.Now().UTC().Truncate(time.Hour).Add(-6 * time.Hour)
	snapB := ids.New()
	insertSnapshot(ctx, t, pool, snapB, h2.Add(-time.Hour), h2)
	validObs := ids.New()
	insertObservation(ctx, t, pool, validObs, h2, "reanalysis", "valid", nil)

	m := newMatcher(pool)
	created, err := m.MatchBatch(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, created, "only the valid-observation snapshot matches")
	assert.Equal(t, 1, pairCount(ctx, t, pool))

	// The pair is snapshot B → the valid observation; the suspect snapshot A has none.
	var chosen, snap uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT forecast_snapshot_id, observation_id FROM matched_evaluations`).Scan(&snap, &chosen))
	assert.Equal(t, snapB, snap)
	assert.Equal(t, validObs, chosen)

	// Idempotent: a second batch creates zero new pairs.
	created, err = m.MatchBatch(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, created)
	assert.Equal(t, 1, pairCount(ctx, t, pool))
}

// TestMatching_RematchOnCorrection proves the correction cascade (workflow §5):
// when a matched observation is superseded, the next batch adds a NEW pair to
// the correcting observation and retains the old pair (lineage).
func TestMatching_RematchOnCorrection(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	pool := newPool(ctx, t, connStr)
	(&testEnv{pool: pool}).seedCatalog(ctx, t)

	target := time.Now().UTC().Truncate(time.Hour).Add(-5 * time.Hour)
	issued := target.Add(-time.Hour)
	snapID := ids.New()
	insertSnapshot(ctx, t, pool, snapID, issued, target)
	origObs := ids.New()
	insertObservation(ctx, t, pool, origObs, target, "reanalysis", "valid", nil)

	m := newMatcher(pool)
	created, err := m.MatchBatch(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, created)

	// A correction arrives: insert the corrected row, then supersede the original
	// (mirrors ObserveService supersede-then-insert; the live index tolerates it).
	correctedObs := ids.New()
	_, err = pool.Exec(ctx,
		`UPDATE observations SET superseded_observation_id = $2 WHERE id = $1`, origObs, correctedObs)
	require.NoError(t, err)
	insertObservation(ctx, t, pool, correctedObs, target, "reanalysis", "corrected", nil)

	// Next batch rematches: a new pair to the corrected observation; old retained.
	created, err = m.MatchBatch(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, created, "one rematch pair created")
	assert.Equal(t, 2, pairCount(ctx, t, pool), "old pair retained for lineage")

	// Both pairs exist for the snapshot; the new one points to the corrected obs.
	var toCorrected int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM matched_evaluations WHERE forecast_snapshot_id = $1 AND observation_id = $2`,
		snapID, correctedObs).Scan(&toCorrected))
	assert.Equal(t, 1, toCorrected)

	// Idempotent: re-running does not duplicate the rematch.
	created, err = m.MatchBatch(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, created)
	assert.Equal(t, 2, pairCount(ctx, t, pool))
}
