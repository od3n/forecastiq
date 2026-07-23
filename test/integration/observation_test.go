//go:build integration

package integration

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/adapters/persistence/observationpg"
	"github.com/forecastiq/forecastiq/internal/catalog"
	catalogdomain "github.com/forecastiq/forecastiq/internal/catalog/domain"
	"github.com/forecastiq/forecastiq/internal/collection"
	collectiondomain "github.com/forecastiq/forecastiq/internal/collection/domain"
	"github.com/forecastiq/forecastiq/internal/collection/ports"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
	"github.com/forecastiq/forecastiq/internal/platform/events"
	"github.com/forecastiq/forecastiq/internal/platform/ids"
	"github.com/forecastiq/forecastiq/internal/platform/metrics"
)

// slog_discard returns a logger that discards output (reuses the harness sink).
func slog_discard() *slog.Logger { return slog.New(slog.NewTextHandler(io_discard{}, nil)) }

// obsSource is the observation source used across these tests.
const obsSource = "openmeteo_historical"

// stubObsAdapter returns a fixed observation set, letting the test drive the
// dedup/correction cascade deterministically against a real database.
type stubObsAdapter struct {
	obs []*collectiondomain.Observation
}

func (s *stubObsAdapter) Source() string         { return obsSource }
func (s *stubObsAdapter) SchemaVersion() string  { return "openmeteo-historical-v1" }
func (s *stubObsAdapter) AdapterVersion() string { return "1.0.0-test" }
func (s *stubObsAdapter) FetchObservations(_ context.Context, _ ports.ObservationRequest) (*ports.ObservationResult, error) {
	out := &ports.ObservationResult{
		Source: obsSource, SchemaVersion: "openmeteo-historical-v1", AdapterVersion: "1.0.0-test",
		Outcome: ports.OutcomeSuccess, RecordsReceived: len(s.obs), Observations: s.obs,
	}
	for _, o := range s.obs {
		if o.QualityFlag == collectiondomain.QualitySuspect {
			out.SuspectCount++
		}
	}
	return out, nil
}

func obsF64(v float64) *float64 { return &v }

// obsAt builds a fresh reanalysis observation for the JB location at t.
func obsAt(t time.Time, temp float64, flag collectiondomain.QualityFlag) *collectiondomain.Observation {
	return &collectiondomain.Observation{
		ID:              ids.New(),
		LocationID:      catalogdomain.JohorBahruLocationID,
		Source:          obsSource,
		ObservationType: collectiondomain.ObservationReanalysis,
		ObservedAt:      t.UTC(),
		TemperatureC:    obsF64(temp),
		HumidityPct:     obsF64(70),
		QualityFlag:     flag,
	}
}

func newObserver(pool *pgxpool.Pool, adapter ports.ObservationSourceAdapter, bus events.Bus) *collection.ObserveService {
	logger := slog_discard()
	return collection.NewObserveService(adapter, observationpg.NewRepository(), bus, metrics.New(),
		clock.Real{}, logger, dbtx.NewRunner(pool), pool)
}

func jbLocation() *catalog.Location {
	return &catalog.Location{
		ID: catalogdomain.JohorBahruLocationID, Latitude: 1.4927, Longitude: 103.7414,
		Timezone: "Asia/Kuala_Lumpur", Status: catalogdomain.StatusActive,
	}
}

// TestObservationCollection_DedupAndCorrection proves the WP-10 pipeline against
// real PostgreSQL: window-backfill dedup (OC-03), correction detection →
// supersession (the one permitted mutation) with the corrected row coexisting
// under the partial live-row index, and the corrected event.
func TestObservationCollection_DedupAndCorrection(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	pool := newPool(ctx, t, connStr)
	env := &testEnv{pool: pool}
	env.seedCatalog(ctx, t)

	h9 := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)
	h10 := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	loc := jbLocation()

	var corrected int
	bus := events.NewSyncBus(slog_discard())
	bus.Subscribe("observation.corrected", func(_ context.Context, _ events.Event) { corrected++ })

	// 1) First collection: two fresh rows stored.
	obs := newObserver(pool, &stubObsAdapter{obs: []*collectiondomain.Observation{
		obsAt(h9, 30.5, collectiondomain.QualityValid),
		obsAt(h10, 31.2, collectiondomain.QualityValid),
	}}, bus)
	stored, err := obs.Observe(ctx, loc, h9, h10)
	require.NoError(t, err)
	assert.Equal(t, 2, stored)
	assert.Equal(t, 2, liveObservationCount(ctx, t, pool))

	// 2) Re-collect the identical window: everything deduplicates (0 stored).
	stored, err = obs.Observe(ctx, loc, h9, h10)
	require.NoError(t, err)
	assert.Equal(t, 0, stored, "unchanged re-fetch deduplicates")
	assert.Equal(t, 2, liveObservationCount(ctx, t, pool))

	// 3) Re-collect with h10 changed beyond ε → one correction.
	obs2 := newObserver(pool, &stubObsAdapter{obs: []*collectiondomain.Observation{
		obsAt(h9, 30.5, collectiondomain.QualityValid),  // unchanged → dedup
		obsAt(h10, 32.4, collectiondomain.QualityValid), // +1.2°C → correction
	}}, bus)
	stored, err = obs2.Observe(ctx, loc, h9, h10)
	require.NoError(t, err)
	assert.Equal(t, 1, stored, "only the corrected row is stored")
	assert.Equal(t, 1, corrected, "observation.corrected emitted once")

	// Live rows still 2 (h9 + corrected h10); total 3 (old h10 superseded).
	assert.Equal(t, 2, liveObservationCount(ctx, t, pool))
	assert.Equal(t, 3, totalObservationCount(ctx, t, pool))
	assert.Equal(t, 1, supersededCount(ctx, t, pool), "old h10 row superseded")

	temp, flag := liveRowAt(ctx, t, pool, h10)
	assert.InDelta(t, 32.4, temp, 0.001)
	assert.Equal(t, "corrected", flag)
}

// TestObservationCollection_SuspectStored proves OC-04 suspect rows are stored
// (not dropped) with quality_flag='suspect'.
func TestObservationCollection_SuspectStored(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	pool := newPool(ctx, t, connStr)
	env := &testEnv{pool: pool}
	env.seedCatalog(ctx, t)

	h9 := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)
	obs := newObserver(pool, &stubObsAdapter{obs: []*collectiondomain.Observation{
		obsAt(h9, 28.0, collectiondomain.QualitySuspect),
	}}, events.NewSyncBus(slog_discard()))
	stored, err := obs.Observe(ctx, jbLocation(), h9, h9)
	require.NoError(t, err)
	assert.Equal(t, 1, stored)

	_, flag := liveRowAt(ctx, t, pool, h9)
	assert.Equal(t, "suspect", flag, "suspect row stored, not dropped")
}

// ── query helpers ─────────────────────────────────────────────────────

func liveObservationCount(ctx context.Context, t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM observations WHERE superseded_observation_id IS NULL`).Scan(&n))
	return n
}

func totalObservationCount(ctx context.Context, t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM observations`).Scan(&n))
	return n
}

func supersededCount(ctx context.Context, t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM observations WHERE superseded_observation_id IS NOT NULL`).Scan(&n))
	return n
}

func liveRowAt(ctx context.Context, t *testing.T, pool *pgxpool.Pool, observedAt time.Time) (float64, string) {
	t.Helper()
	var temp float64
	var flag string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT temperature_c, quality_flag FROM observations
		 WHERE observed_at = $1 AND superseded_observation_id IS NULL`, observedAt.UTC()).Scan(&temp, &flag))
	return temp, flag
}
