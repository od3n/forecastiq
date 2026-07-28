// Command pt3 is the PT-3 ingestion-burst harness (WP-26b; performance doc
// §2): it drives 240 collections — a simulated day (24 hourly model runs × 10
// locations) — through the REAL CollectService (classify → partition → tx →
// snapshot COPY/dedup → payload store → events) using an in-process fake
// provider adapter, per the doc's "via fake provider".
//
// Gates:
//   - cycle completion: total wall time < --budget (default 5m, NFR-P07)
//   - snapshot write p95: per-collection service latency < --p95 (default
//     100ms). The measured span is CollectService.Collect, which brackets the
//     snapshot write; the fake provider itself responds in-process (~0ms).
//
// Usage:
//
//	go run ./test/perf/pt3 --db "$FIQ_DATABASE_URL" [--collections=240]
//
// The target DB must be migrated and seeded with the perf catalog (the base
// preset provides the 10 locations). Rows land under the Open-Meteo provider
// with synthetic model-run times far in the past (2 years back) so they never
// collide with — or shadow — seeded or live forecasts.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/forecastiq/forecastiq/adapters/payloadstore"
	"github.com/forecastiq/forecastiq/adapters/persistence/auditpg"
	"github.com/forecastiq/forecastiq/adapters/persistence/catalogpg"
	"github.com/forecastiq/forecastiq/adapters/persistence/collectionpg"
	"github.com/forecastiq/forecastiq/internal/audit"
	"github.com/forecastiq/forecastiq/internal/catalog"
	catalogdomain "github.com/forecastiq/forecastiq/internal/catalog/domain"
	"github.com/forecastiq/forecastiq/internal/collection"
	collectiondomain "github.com/forecastiq/forecastiq/internal/collection/domain"
	"github.com/forecastiq/forecastiq/internal/collection/ports"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
	"github.com/forecastiq/forecastiq/internal/platform/events"
	"github.com/forecastiq/forecastiq/internal/platform/metrics"
	"github.com/forecastiq/forecastiq/test/perf/perfids"
)

var (
	dbURL       = flag.String("db", "", "database URL (default: FIQ_DATABASE_URL env)")
	collections = flag.Int("collections", 240, "number of collections in the burst")
	nLocations  = flag.Int("locations", 10, "perf locations to spread the burst across")
	budget      = flag.Duration("budget", 5*time.Minute, "cycle-completion gate (NFR-P07)")
	p95Gate     = flag.Duration("p95", 100*time.Millisecond, "per-collection p95 gate (snapshot write path)")
	rowsPerRun  = flag.Int("rows", 104, "forecast rows per collection run")
)

func main() {
	flag.Parse()
	if *dbURL == "" {
		*dbURL = os.Getenv("FIQ_DATABASE_URL")
	}
	if *dbURL == "" {
		fmt.Fprintln(os.Stderr, "ERROR: --db or FIQ_DATABASE_URL required")
		os.Exit(1)
	}
	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "PT-3 FAILED: %v\n", err)
		os.Exit(1)
	}
}

// burstAdapter produces a deterministic 104-row forecast per collection with a
// DISTINCT simulated model run per call, so every run inserts fresh snapshot
// rows instead of hitting the (provider, location, issued_at, target_time)
// dedup boundary — a faithful "simulated day".
type burstAdapter struct {
	run  int
	base time.Time // anchor issuance (2 years back, hour-truncated)
	rows int
}

func (a *burstAdapter) Slug() string           { return "open-meteo" }
func (a *burstAdapter) SchemaVersion() string  { return "perf-burst-v1" }
func (a *burstAdapter) AdapterVersion() string { return "1.0.0-perf" }
func (a *burstAdapter) Capabilities() ports.Capabilities {
	return ports.Capabilities{MaxForecastHorizon: 7 * 24 * time.Hour, HourlyResolution: true}
}

func (a *burstAdapter) FetchForecast(_ context.Context, req ports.ForecastRequest) (*ports.ForecastResult, error) {
	a.run++
	issued := a.base.Add(time.Duration(a.run) * time.Hour)
	raw := []byte(fmt.Sprintf(`{"perf":"burst","run":%d}`, a.run))
	res := &ports.ForecastResult{
		RawPayload: raw, Checksum: ports.Checksum(raw),
		HTTPStatusCode: 200, LatencyMS: 1,
		SchemaVersion: a.SchemaVersion(), AdapterVersion: a.AdapterVersion(),
		IssuedAt: issued, ModelRunTime: &issued,
		RecordsReceived: a.rows, Outcome: ports.OutcomeSuccess,
	}
	for i := 0; i < a.rows; i++ {
		temp := 27.0 + float64(i%8)
		res.Snapshots = append(res.Snapshots, &collectiondomain.ForecastSnapshot{
			ID:                       uuid.Must(uuid.NewV7()),
			ProviderID:               req.ProviderID,
			LocationID:               req.LocationID,
			IssuedAt:                 issued,
			TargetTime:               issued.Add(time.Duration(i+1) * time.Hour),
			ForecastHorizonMinutes:   (i + 1) * 60,
			TemperatureC:             &temp,
			CanonicalConditionCode:   collectiondomain.ConditionPartlyCloudy,
			ConditionTaxonomyVersion: collectiondomain.ConditionTaxonomyVersion,
		})
	}
	return res, nil
}

func run(ctx context.Context) error {
	pool, err := pgxpool.New(ctx, *dbURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()

	tx := dbtx.NewRunner(pool)
	clk := clock.Real{}
	logger := slog.New(slog.NewTextHandler(discard{}, nil))

	providers := catalog.NewProviderService(catalogpg.NewProviderRepository(), pool, tx,
		audit.NewRecorder(auditpg.NewStore()), clk)
	configs := catalog.NewConfigurationService(catalogpg.NewConfigurationRepository(), pool, tx,
		audit.NewRecorder(auditpg.NewStore()), clk)
	circuits := catalog.NewCircuitService(catalogpg.NewCircuitRepository(), tx)

	store, err := payloadstore.NewFilesystemStore(os.TempDir() + "/fiq-pt3-payloads")
	if err != nil {
		return fmt.Errorf("payload store: %w", err)
	}

	// Anchor the simulated day 2 years back so burst rows sit in retired
	// partitions and can never shadow seeded/live forecasts in serving reads.
	anchor := time.Now().UTC().Truncate(time.Hour).AddDate(-2, 0, 0)
	adapter := &burstAdapter{base: anchor, rows: *rowsPerRun}

	collector := collection.NewCollectService(
		map[string]ports.ForecastProviderAdapter{"open-meteo": adapter},
		providers, collectionpg.NewCollectionRepository(), collectionpg.NewSnapshotRepository(),
		store, circuits, events.NewSyncBus(logger), metrics.New(),
		audit.NewRecorder(auditpg.NewStore()), clk, logger, tx, pool,
		func(string) string { return "" })

	provider, err := providers.GetProvider(ctx, catalogdomain.OpenMeteoProviderID)
	if err != nil {
		return fmt.Errorf("open-meteo provider missing (seed the perf catalog first): %w", err)
	}
	config, err := configs.GetConfigurationByProviderID(ctx, catalogdomain.OpenMeteoProviderID)
	if err != nil {
		return fmt.Errorf("open-meteo configuration missing: %w", err)
	}

	locs := make([]*catalogdomain.Location, 0, *nLocations)
	locService := catalog.NewLocationService(catalogpg.NewLocationRepository(), tx, pool,
		audit.NewRecorder(auditpg.NewStore()), clk, logger)
	for i := 0; i < *nLocations; i++ {
		loc, lerr := locService.GetLocation(ctx, perfids.LocationID(i))
		if lerr != nil {
			return fmt.Errorf("perf location %d missing (run the seeder first): %w", i, lerr)
		}
		locs = append(locs, loc)
	}

	fmt.Printf("=== PT-3 Ingestion Burst ===\n")
	fmt.Printf("Collections: %d across %d locations (%d rows each)\n", *collections, len(locs), *rowsPerRun)
	fmt.Printf("Gates: cycle < %s (NFR-P07); per-collection p95 < %s\n\n", *budget, *p95Gate)

	durations := make([]time.Duration, 0, *collections)
	statuses := map[collectiondomain.CollectionStatus]int{}
	start := time.Now()
	for i := 0; i < *collections; i++ {
		loc := locs[i%len(locs)]
		t0 := time.Now()
		coll, cerr := collector.Collect(ctx, collection.CollectInput{
			Provider: provider, Location: loc, Config: config,
			Actor: catalog.Actor{Name: "pt3-harness"}, Source: collection.SourceScheduled,
		})
		if cerr != nil {
			return fmt.Errorf("collection %d: %w", i, cerr)
		}
		durations = append(durations, time.Since(t0))
		statuses[coll.Status]++
	}
	elapsed := time.Since(start)

	sort.Slice(durations, func(a, b int) bool { return durations[a] < durations[b] })
	q := func(p float64) time.Duration { return durations[int(p*float64(len(durations)-1))] }

	fmt.Printf("Cycle time:  %s (%d collections)\n", elapsed.Round(time.Millisecond), *collections)
	fmt.Printf("Statuses:    %v\n", statuses)
	fmt.Printf("Latency:     p50=%s p95=%s p99=%s max=%s\n\n",
		q(0.50).Round(time.Millisecond), q(0.95).Round(time.Millisecond),
		q(0.99).Round(time.Millisecond), q(1.0).Round(time.Millisecond))

	failed := false
	if elapsed >= *budget {
		fmt.Printf("[FAIL] cycle completion %s >= %s (NFR-P07)\n", elapsed.Round(time.Millisecond), *budget)
		failed = true
	} else {
		fmt.Printf("[PASS] cycle completion %s < %s (NFR-P07)\n", elapsed.Round(time.Millisecond), *budget)
	}
	if q(0.95) >= *p95Gate {
		fmt.Printf("[FAIL] per-collection p95 %s >= %s\n", q(0.95).Round(time.Millisecond), *p95Gate)
		failed = true
	} else {
		fmt.Printf("[PASS] per-collection p95 %s < %s\n", q(0.95).Round(time.Millisecond), *p95Gate)
	}
	if statuses[collectiondomain.StatusSuccess] != *collections {
		fmt.Printf("[FAIL] %d/%d collections succeeded\n", statuses[collectiondomain.StatusSuccess], *collections)
		failed = true
	} else {
		fmt.Printf("[PASS] %d/%d collections succeeded\n", *collections, *collections)
	}
	if failed {
		return fmt.Errorf("threshold gate breached")
	}
	return nil
}

// discard is a no-op slog sink.
type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
