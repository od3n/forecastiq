//go:build integration

package integration

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/adapters/persistence/schedulerpg"
	catalogdomain "github.com/forecastiq/forecastiq/internal/catalog/domain"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
	"github.com/forecastiq/forecastiq/internal/platform/metrics"
	"github.com/forecastiq/forecastiq/internal/scheduler"
)

// generateDueSlot inserts one due forecast slot at slotTime.
func generateDueSlot(ctx context.Context, t *testing.T, e *testEnv, slotTime time.Time) {
	t.Helper()
	locID := catalogdomain.JohorBahruLocationID
	require.NoError(t, e.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		return e.slots.Generate(ctx, tx, &scheduler.Slot{
			ID: mustUUIDv7(), ProviderConfigurationID: catalogdomain.OpenMeteoConfigID,
			JobType: scheduler.JobForecastCollection, LocationID: &locID,
			SlotTime: slotTime, Status: scheduler.SlotDue,
			CreatedAt: slotTime, UpdatedAt: slotTime,
		})
	}))
}

// TestLeaseExpiryReclaim proves a claimed slot whose lease has expired is
// reclaimed by another instance (crash recovery; workflow 05 §3).
func TestLeaseExpiryReclaim(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	now := time.Now().UTC()
	generateDueSlot(ctx, t, e, now.Add(-time.Minute))

	claim := func(instance string, at time.Time, lease time.Duration) []*scheduler.Slot {
		var got []*scheduler.Slot
		require.NoError(t, e.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
			var err error
			got, err = e.slots.ClaimDue(ctx, tx, instance, at, lease, 10)
			return err
		}))
		return got
	}

	// Instance A claims with a short lease.
	first := claim("instance-a", now, 50*time.Millisecond)
	require.Len(t, first, 1)
	assert.Equal(t, "instance-a", first[0].ClaimedBy)

	// Instance B cannot claim while the lease is valid.
	assert.Empty(t, claim("instance-b", now, 5*time.Minute))

	// After the lease expires, instance B reclaims the same slot.
	reclaimed := claim("instance-b", now.Add(time.Second), 5*time.Minute)
	require.Len(t, reclaimed, 1)
	assert.Equal(t, first[0].ID, reclaimed[0].ID)
	assert.Equal(t, "instance-b", reclaimed[0].ClaimedBy)
}

// newScheduler builds a scheduler over the test env with a fresh metric set the
// caller can assert against.
func newScheduler(t *testing.T, e *testEnv, cfg scheduler.Config) (*scheduler.Scheduler, *metrics.Metrics) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io_discard{}, nil))
	m := metrics.New()
	dispatcher := scheduler.NewForecastDispatcher(e.configs, e.providers, e.locations, e.collector, logger)
	s := scheduler.New(e.slots, schedulerpg.NewRunRepository(), dispatcher,
		e.configs, e.locations, e.tx, clock.Real{}, logger, m, cfg)
	return s, m
}

// TestSchedulerRunCollectsAndDrains runs the real scheduler loop: it generates
// and claims due slots, executes a collection, records scheduler metrics
// (claimed + lag + missed), and drains cleanly on context cancellation.
func TestSchedulerRunCollectsAndDrains(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(3))
	e.seedCatalog(ctx, t)

	s, m := newScheduler(t, e, scheduler.Config{
		Interval: 50 * time.Millisecond, LeaseDuration: time.Minute,
		MissedThreshold: time.Millisecond, DrainTimeout: 5 * time.Second, JobTimeout: 10 * time.Second,
	})

	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); s.Run(runCtx) }()

	// A collection lands once the generated slot is claimed and executed.
	require.Eventually(t, func() bool {
		var n int
		_ = e.pool.QueryRow(ctx,
			`SELECT count(*) FROM forecast_collections WHERE collection_status = 'success'`).Scan(&n)
		return n >= 1
	}, 5*time.Second, 25*time.Millisecond, "scheduler should produce a successful collection")

	// Cancel and confirm the loop drains and returns promptly.
	cancel()
	select {
	case <-done:
	case <-time.After(6 * time.Second):
		t.Fatal("scheduler did not drain/stop within deadline")
	}

	assert.Positive(t, testutil.ToFloat64(m.SlotsClaimed.WithLabelValues(scheduler.JobForecastCollection)),
		"slots claimed counter should advance")
	assert.Positive(t, testutil.ToFloat64(m.MissedSlots.WithLabelValues(scheduler.JobForecastCollection)),
		"overdue generated slots should count as missed")

	// Exactly the 3 snapshots exist — repeated slot execution never duplicates.
	var snaps int
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT count(*) FROM forecast_snapshots WHERE location_id = $1`,
		catalogdomain.JohorBahruLocationID).Scan(&snaps))
	assert.Equal(t, 3, snaps)
}

// TestSchedulerNoDoubleCollection runs two scheduler instances against the same
// database and asserts the collection acceptance criterion: a double-fire
// produces zero duplicate snapshots (SKIP LOCKED claim + collection dedup).
func TestSchedulerNoDoubleCollection(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(5))
	e.seedCatalog(ctx, t)

	cfg := scheduler.Config{Interval: 40 * time.Millisecond, LeaseDuration: time.Minute,
		DrainTimeout: 5 * time.Second, JobTimeout: 10 * time.Second}
	s1, _ := newScheduler(t, e, cfg)
	s2, _ := newScheduler(t, e, cfg)

	runCtx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); s1.Run(runCtx) }()
	go func() { defer wg.Done(); s2.Run(runCtx) }()

	require.Eventually(t, func() bool {
		var n int
		_ = e.pool.QueryRow(ctx,
			`SELECT count(*) FROM forecast_snapshots WHERE location_id = $1`,
			catalogdomain.JohorBahruLocationID).Scan(&n)
		return n >= 5
	}, 5*time.Second, 25*time.Millisecond)

	cancel()
	wg.Wait()

	var snaps int
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT count(*) FROM forecast_snapshots WHERE location_id = $1`,
		catalogdomain.JohorBahruLocationID).Scan(&snaps))
	assert.Equal(t, 5, snaps, "two instances must not double-collect")
}
