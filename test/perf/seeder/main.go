// Package main is the synthetic data seeder for performance testing (WP-26 /
// WP-26b). It generates deterministic, physically plausible tropical weather
// data for the ForecastIQ database, enabling reproducible performance
// measurements.
//
// Reference: docs/testing/04-performance-testing.md §3
//
// Usage:
//
//	go run ./test/perf/seeder --locations=10 --providers=2 --days=30
//	go run ./test/perf/seeder --preset=base     # 10 loc × 2 prov × 30d
//	go run ./test/perf/seeder --preset=extended # 2× MVP annual rate (PT-7)
//	go run ./test/perf/seeder --preset=analysis # 100K-pair PT-4 input
//	go run ./test/perf/seeder --estimate-only   # print volumes, write nothing
//	go run ./test/perf/seeder --reset           # TRUNCATE perf data first
//
// The target database must be migrated (the seeder creates historical monthly
// partitions itself via create_monthly_partition). Catalog writes are
// insert-only; data tables must be empty (or --reset given) so COPY cannot
// collide with prior rows.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	preset       = flag.String("preset", "base", "data preset: base|extended|analysis")
	locations    = flag.Int("locations", 10, "number of locations")
	providers    = flag.Int("providers", 2, "number of providers")
	days         = flag.Int("days", 30, "days of historical data")
	seed         = flag.Int64("seed", 42, "random seed for deterministic generation")
	dbURL        = flag.String("db", "", "database URL (default: FIQ_DATABASE_URL env)")
	estimateOnly = flag.Bool("estimate-only", false, "print volume estimates and exit 0 without writing")
	reset        = flag.Bool("reset", false, "TRUNCATE perf data tables before seeding (catalog is kept)")
)

// Fan-out constants per docs/testing/04-performance-testing.md §3:
// base (10 loc × 2 prov × 30 d) ≈ 1.5M snapshots ⇒ ~2500 snapshot rows per
// provider-location-day = hourly collections × forecast-horizon rows.
const (
	collectionsPerDay      = 24  // hourly collection runs
	forecastRowsPerRun     = 104 // hourly to +72h, 3-hourly to +168h (targetOffsets)
	matchesPerSnapshotX100 = 200 // doc §3: ~2× snapshots (original + rematch pair)
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

	// Apply preset overrides
	switch *preset {
	case "base":
		*locations, *providers, *days = 10, 2, 30
	case "extended":
		// 2× MVP annual rate (doc §3, PT-7 NFR-S01) ≈ 35M snapshots.
		*locations, *providers, *days = 20, 2, 365
	case "analysis":
		// ≈ 100K pairs in the current month for PT-4 (NFR-P06): pairs/day ≈
		// loc × prov × 24 issuances × ~104 observed targets ⇒ 2 loc × 2 prov
		// × 10 d ≈ 100K snapshot-pair chains inside the 30 d match window.
		*locations, *providers, *days = 2, 2, 10
	}

	// Redact credentials before printing (DRB-WP26-003): the raw URL contains
	// the DB password, which would land in CI logs.
	redacted := *dbURL
	if u, err := url.Parse(*dbURL); err == nil {
		redacted = u.Redacted()
	}

	fmt.Printf("=== ForecastIQ Performance Seeder ===\n")
	fmt.Printf("Preset:    %s\n", *preset)
	fmt.Printf("Locations: %d\n", *locations)
	fmt.Printf("Providers: %d\n", *providers)
	fmt.Printf("Days:      %d\n", *days)
	fmt.Printf("Seed:      %d\n", *seed)
	fmt.Printf("DB:        %s\n", redacted)
	fmt.Println()

	// Volume estimates per doc §3 fan-out (collections/day × horizon rows).
	collectionDays := *locations * *providers * *days
	snapshots := collectionDays * collectionsPerDay * forecastRowsPerRun
	observations := *locations * *days * collectionsPerDay * 2 // original + correcting row
	matches := snapshots * matchesPerSnapshotX100 / 100
	metrics := *locations * *providers * len(canonicalHorizons) * *days * len(metricPlan)

	fmt.Printf("Estimated volumes:\n")
	fmt.Printf("  Snapshots:    %d\n", snapshots)
	fmt.Printf("  Observations: %d\n", observations)
	fmt.Printf("  Matches:      %d\n", matches)
	fmt.Printf("  Metrics:      %d\n", metrics)
	fmt.Println()

	if *estimateOnly {
		fmt.Println("Estimate-only mode: no rows written.")
		return
	}

	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

// run executes the seeding pipeline against the target database.
func run(ctx context.Context) error {
	start := time.Now()
	conn, err := pgx.Connect(ctx, *dbURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	if *reset {
		fmt.Println("Resetting perf data tables (TRUNCATE)...")
		if rerr := resetData(ctx, conn); rerr != nil {
			return rerr
		}
	}
	present, err := dataPresent(ctx, conn)
	if err != nil {
		return err
	}
	if present {
		return fmt.Errorf("data tables are not empty — pass --reset to truncate perf data first " +
			"(immutability triggers forbid DELETE; never run against operator data)")
	}

	provs := buildProviders(*providers)
	d := newDataset(*seed, provs, *locations, *days, time.Now())

	fmt.Println("Seeding catalog (insert-only)...")
	if err := ensureCatalog(ctx, conn, provs, *locations); err != nil {
		return err
	}
	// Snapshot targets extend 7 d past the last issuance hour.
	if err := ensurePartitions(ctx, conn, d.start, d.end.Add(8*24*time.Hour)); err != nil {
		return err
	}

	total := int64(0)
	for _, step := range []struct {
		name string
		fn   func(context.Context, *pgx.Conn, dataset) (int64, error)
	}{
		{"forecast_collections", copyCollections},
		{"forecast_snapshots", copySnapshots},
		{"observations", copyObservations},
		{"matched_evaluations", copyPairs},
		{"accuracy_metrics", copyMetrics},
		{"provider_rankings", copyRankings},
	} {
		t0 := time.Now()
		n, err := step.fn(ctx, conn, d)
		if err != nil {
			return fmt.Errorf("%s: %w", step.name, err)
		}
		total += n
		fmt.Printf("  %-22s %12d rows  (%s)\n", step.name, n, time.Since(t0).Round(time.Millisecond))
	}

	// A zero-row outcome would mean k6 runs against an empty DB believing it
	// was seeded (DRB-WP26-004) — fail loudly.
	if total == 0 {
		return fmt.Errorf("no rows written")
	}

	fmt.Println()
	fmt.Printf("Seed complete: %d rows in %s\n", total, time.Since(start).Round(time.Millisecond))
	fmt.Printf("First location id (for k6 LOCATION_ID): %s\n", perfLocationID(0))
	return nil
}
