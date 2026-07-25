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
	"os"
	"time"
)

var (
	preset    = flag.String("preset", "base", "data preset: base|extended|analysis")
	locations = flag.Int("locations", 10, "number of locations")
	providers = flag.Int("providers", 2, "number of providers")
	days      = flag.Int("days", 30, "days of historical data")
	seed      = flag.Int64("seed", 42, "random seed for deterministic generation")
	dbURL     = flag.String("db", "", "database URL (default: FIQ_DATABASE_URL env)")
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
		*locations, *providers, *days = 10, 2, 365
	case "analysis":
		*locations, *providers, *days = 5, 2, 60
	}

	rng := rand.New(rand.NewSource(*seed))

	fmt.Printf("=== ForecastIQ Performance Seeder ===\n")
	fmt.Printf("Preset:    %s\n", *preset)
	fmt.Printf("Locations: %d\n", *locations)
	fmt.Printf("Providers: %d\n", *providers)
	fmt.Printf("Days:      %d\n", *days)
	fmt.Printf("Seed:      %d\n", *seed)
	fmt.Printf("DB:        %s...\n", (*dbURL)[:min(40, len(*dbURL))])
	fmt.Println()

	start := time.Now()

	// Generate data volumes
	snapshots := *locations * *providers * *days * 24 // hourly snapshots
	observations := *locations * *days * 24           // hourly observations
	matches := snapshots                              // 1:1 at best
	metrics := *locations * *providers * *days / 7    // weekly metrics

	fmt.Printf("Estimated volumes:\n")
	fmt.Printf("  Snapshots:    %d\n", snapshots)
	fmt.Printf("  Observations: %d\n", observations)
	fmt.Printf("  Matches:      %d\n", matches)
	fmt.Printf("  Metrics:      %d\n", metrics)
	fmt.Println()

	// TODO: Connect to DB and generate actual rows.
	// For now, this is the scaffold that documents the seeder interface
	// and volume calculations. The actual INSERT logic requires importing
	// the repository layer which is in internal/ (use pgx directly here).
	_ = rng
	fmt.Printf("Seeder scaffold complete (actual generation requires DB connection).\n")
	fmt.Printf("Duration: %s\n", time.Since(start))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
