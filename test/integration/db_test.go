//go:build integration

package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/adapters/persistence/collectionpg"
	"github.com/forecastiq/forecastiq/internal/catalog"
	catalogdomain "github.com/forecastiq/forecastiq/internal/catalog/domain"
	"github.com/forecastiq/forecastiq/internal/collection"
	collectiondomain "github.com/forecastiq/forecastiq/internal/collection/domain"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
	"github.com/forecastiq/forecastiq/internal/scheduler"
)

func collectInput(ctx context.Context, t *testing.T, e *testEnv) collection.CollectInput {
	t.Helper()
	provider, err := e.providers.GetProvider(ctx, catalogdomain.OpenMeteoProviderID)
	require.NoError(t, err)
	location, err := e.locations.GetLocation(ctx, catalogdomain.JohorBahruLocationID)
	require.NoError(t, err)
	config, err := e.configs.GetConfigurationByProviderID(ctx, catalogdomain.OpenMeteoProviderID)
	require.NoError(t, err)
	return collection.CollectInput{
		Provider: provider, Location: location, Config: config,
		Actor: catalog.Actor{Name: "test"}, Source: collection.SourceManual,
	}
}

func TestMigrationsApply(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	pool := newPool(ctx, t, connStr)

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables WHERE table_name = 'forecast_snapshots'`).Scan(&count))
	assert.Equal(t, 1, count)
}

func TestCollectionIdempotency(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(3))
	e.seedCatalog(ctx, t)

	in := collectInput(ctx, t, e)

	first, err := e.collector.Collect(ctx, in)
	require.NoError(t, err)
	assert.Equal(t, collectiondomain.StatusSuccess, first.Status)
	assert.Equal(t, 3, first.SnapshotsStored)

	// Second collection in the same hour → collection-level dedup.
	second, err := e.collector.Collect(ctx, in)
	require.NoError(t, err)
	assert.Equal(t, collectiondomain.StatusDeduplicated, second.Status)
	assert.Equal(t, 0, second.SnapshotsStored)

	// Only 3 snapshots exist (no duplicates).
	snaps, err := e.reader.SnapshotsByCollection(ctx, first.ID)
	require.NoError(t, err)
	assert.Len(t, snaps, 3)
}

func TestSnapshotDedupOnConflict(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(3))
	e.seedCatalog(ctx, t)

	// Build a collection + snapshots, then insert the same snapshots twice.
	coll := &collectiondomain.ForecastCollection{
		ID: mustUUIDv7(), ProviderID: catalogdomain.OpenMeteoProviderID,
		LocationID: catalogdomain.JohorBahruLocationID, ProviderConfigurationID: catalogdomain.OpenMeteoConfigID,
		RequestedAt: time.Now().UTC(), Status: collectiondomain.StatusPending, CreatedAt: time.Now().UTC(),
	}
	issued := time.Now().UTC().Truncate(time.Hour)
	var snaps []*collectiondomain.ForecastSnapshot
	for i := 0; i < 3; i++ {
		temp := 25.0 + float64(i)
		snaps = append(snaps, &collectiondomain.ForecastSnapshot{
			ID: mustUUIDv7(), ForecastCollectionID: coll.ID,
			ProviderID: coll.ProviderID, LocationID: coll.LocationID,
			IssuedAt: issued, TargetTime: issued.Add(time.Duration(i+1) * time.Hour),
			ForecastHorizonMinutes: (i + 1) * 60, TemperatureC: &temp,
		})
	}
	snapshotRepo := collectionpg.NewSnapshotRepository()
	collectionRepo := collectionpg.NewCollectionRepository()
	require.NoError(t, e.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		return collectionRepo.Insert(ctx, tx, coll)
	}))
	require.NoError(t, snapshotRepo.EnsurePartitions(ctx, e.pool, monthStartsOf(snaps)))

	stored1, err := snapshotRepo.InsertBatch(ctx, e.pool, snaps)
	require.NoError(t, err)
	assert.Equal(t, 3, stored1)

	// Re-insert identical snapshots → all deduplicated (0 stored).
	stored2, err := snapshotRepo.InsertBatch(ctx, e.pool, snaps)
	require.NoError(t, err)
	assert.Equal(t, 0, stored2)
}

func TestImmutabilityTriggers(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(2))
	e.seedCatalog(ctx, t)

	coll, err := e.collector.Collect(ctx, collectInput(ctx, t, e))
	require.NoError(t, err)
	require.Equal(t, collectiondomain.StatusSuccess, coll.Status)

	// Updating a completed collection is forbidden.
	_, err = e.pool.Exec(ctx, `UPDATE forecast_collections SET records_received = 999 WHERE id = $1`, coll.ID)
	assert.Error(t, err, "completed collection must be immutable")

	// Deleting a collection is forbidden.
	_, err = e.pool.Exec(ctx, `DELETE FROM forecast_collections WHERE id = $1`, coll.ID)
	assert.Error(t, err)

	// Snapshots are fully immutable.
	_, err = e.pool.Exec(ctx, `UPDATE forecast_snapshots SET temperature_c = 0 WHERE forecast_collection_id = $1`, coll.ID)
	assert.Error(t, err, "snapshots must be immutable")
	_, err = e.pool.Exec(ctx, `DELETE FROM forecast_snapshots WHERE forecast_collection_id = $1`, coll.ID)
	assert.Error(t, err)
}

func TestSkipLockedClaim(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	const n = 10
	now := time.Now().UTC()
	// Generate n due slots.
	require.NoError(t, e.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		for i := 0; i < n; i++ {
			locID := catalogdomain.JohorBahruLocationID
			slot := &scheduler.Slot{
				ID: mustUUIDv7(), ProviderConfigurationID: catalogdomain.OpenMeteoConfigID,
				JobType: scheduler.JobForecastCollection, LocationID: &locID,
				SlotTime: now.Add(-time.Duration(i) * time.Minute), Status: scheduler.SlotDue,
				CreatedAt: now, UpdatedAt: now,
			}
			if err := e.slots.Generate(ctx, tx, slot); err != nil {
				return err
			}
		}
		return nil
	}))

	// Two concurrent claimers must not double-claim.
	var wg sync.WaitGroup
	claimed := make(chan []string, 2)
	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			var got []string
			_ = e.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
				slots, err := e.slots.ClaimDue(ctx, tx, id, now, 5*time.Minute, n)
				if err != nil {
					return err
				}
				for _, s := range slots {
					got = append(got, s.ID.String())
				}
				return nil
			})
			claimed <- got
		}(string(rune('a' + w)))
	}
	wg.Wait()
	close(claimed)

	seen := map[string]bool{}
	total := 0
	for ids := range claimed {
		for _, id := range ids {
			assert.False(t, seen[id], "slot %s claimed twice", id)
			seen[id] = true
			total++
		}
	}
	assert.Equal(t, n, total, "all slots claimed exactly once across workers")
}
