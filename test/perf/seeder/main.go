// Package main is the synthetic data seeder for performance testing (WP-26).
// It generates deterministic, physically plausible tropical weather data for
// the ForecastIQ database, enabling reproducible performance measurements.
//
// Reference: docs/testing/04-performance-testing.md §3
//
// Usage:
//
//	go run ./test/perf/seeder --locations=10 --providers=2 --days=30
//	go run ./test/perf/seeder --preset=base     # 10 loc × 2 prov × 30d
//	go run ./test/perf/seeder --preset=extended # 2× MVP annual rate
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"time"
)

var (
	preset       = flag.String("preset", "base", "data preset: base|extended|analysis")
	locations    = flag.Int("locations", 10, "number of locations")
	providers    = flag.Int("providers", 2, "number of providers")
	days         = flag.Int("days", 30, "days of historical data")
	seed         = flag.Int64("seed", 42, "random seed for deterministic generation")
	dbURL        = flag.String("db", "", "database URL (default: FIQ_DATABASE_URL env)")
	estimateOnly = flag.Bool("estimate-only", false, "print volume estimates and exit 0 without writing (row generation is tracked in WP-26b)")
)

// Fan-out constants per docs/testing/04-performance-testing.md §3:
// base (10 loc × 2 prov × 30 d) ≈ 1.5M snapshots ⇒ ~2500 snapshot rows per
// provider-location-day = hourly collections × forecast-horizon rows.
const (
	collectionsPerDay      = 24  // hourly collection runs
	forecastRowsPerRun     = 104 // hourly forecast rows across the horizon
	matchesPerSnapshotX100 = 200 // doc §3: ~2× snapshots (incl. rematch/sub-hourly)
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
		*locations, *providers, *days = 5, 2, 60
	}

	rng := rand.New(rand.NewSource(*seed))

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

	start := time.Now()

	// Volume estimates per doc §3 fan-out (collections/day × horizon rows).
	collectionDays := *locations * *providers * *days
	snapshots := collectionDays * collectionsPerDay * forecastRowsPerRun
	observations := *locations * *days * collectionsPerDay
	matches := snapshots * matchesPerSnapshotX100 / 100
	metrics := *locations * *providers * *days / 7 // weekly metrics

	fmt.Printf("Estimated volumes:\n")
	fmt.Printf("  Snapshots:    %d\n", snapshots)
	fmt.Printf("  Observations: %d\n", observations)
	fmt.Printf("  Matches:      %d\n", matches)
	fmt.Printf("  Metrics:      %d\n", metrics)
	fmt.Println()

	// Row generation is not yet implemented — tracked in WP-26b (see
	// docs/reviews/work-packages/WP-26-delivery-review.md). Exit non-zero by
	// default so a `seeder && k6 run` chain does NOT proceed against an empty
	// DB believing it was seeded (DRB-WP26-004). --estimate-only opts into the
	// scaffold's estimate output for planning.
	_ = rng
	if *estimateOnly {
		fmt.Printf("Estimate-only mode: no rows written. Duration: %s\n", time.Since(start))
		return
	}
	fmt.Fprintln(os.Stderr, "ERROR: row generation not implemented (tracked in WP-26b).")
	fmt.Fprintln(os.Stderr, "Pass --estimate-only to print volumes without writing.")
	os.Exit(1)
}
