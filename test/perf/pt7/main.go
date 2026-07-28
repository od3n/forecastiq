// Command pt7 runs the PT-7 database query baselines (WP-26b; performance doc
// §2): the Q-01 / Q-04 / Q-05 / Q-09 access patterns from
// docs/data/01-query-and-index-requirements.md executed repeatedly with
// rotating parameters against seeded volume, gated at p95 < 100 ms (NFR-P08).
// PT-7 at the extended (2×-volume) preset is the NFR-S01 Level-1 exit gate.
//
// Usage:
//
//	go run ./test/perf/pt7 --db "$FIQ_DATABASE_URL" [--iterations=200]
//
// The DB must hold a seeded perf dataset (base or extended preset).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	catalogdomain "github.com/forecastiq/forecastiq/internal/catalog/domain"
	"github.com/forecastiq/forecastiq/test/perf/perfids"
)

var (
	dbURL      = flag.String("db", "", "database URL (default: FIQ_DATABASE_URL env)")
	iterations = flag.Int("iterations", 200, "executions per query pattern")
	nLocations = flag.Int("locations", 10, "seeded perf locations to rotate across")
	p95Gate    = flag.Duration("p95", 100*time.Millisecond, "p95 gate per pattern (NFR-P08)")
)

// pattern is one benchmarked query shape. run must fully consume the result.
type pattern struct {
	id   string
	desc string
	run  func(ctx context.Context, pool *pgxpool.Pool, i int) error
}

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
		fmt.Fprintf(os.Stderr, "PT-7 FAILED: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	pool, err := pgxpool.New(ctx, *dbURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()

	providers := []uuid.UUID{catalogdomain.OpenMeteoProviderID, catalogdomain.OpenWeatherProviderID}
	horizons := perfids.CanonicalHorizons
	loc := func(i int) uuid.UUID { return perfids.LocationID(i % *nLocations) }

	// drain consumes and discards all rows.
	drain := func(ctx context.Context, pool *pgxpool.Pool, sql string, args ...any) error {
		rows, qerr := pool.Query(ctx, sql, args...)
		if qerr != nil {
			return qerr
		}
		defer rows.Close()
		for rows.Next() {
		}
		return rows.Err()
	}

	patterns := []pattern{
		{
			id:   "Q-01",
			desc: "latest rankings for one location (+ observation context)",
			run: func(ctx context.Context, pool *pgxpool.Pool, i int) error {
				if err := drain(ctx, pool,
					`SELECT pr.provider_id, pr.composite_score, pr.ci_lower, pr.ci_upper,
					        pr.ranking_status, pr.sample_count, pr.coverage, pr.reliability,
					        pr.component_scores, pr.period_start, pr.period_end
					 FROM provider_rankings pr
					 WHERE pr.location_id = $1 AND pr.horizon_minutes = $2 AND pr.horizon_profile = 'uniform'
					   AND pr.superseded_by IS NULL
					   AND pr.period_start = (
					     SELECT max(period_start) FROM provider_rankings
					     WHERE location_id = $1 AND horizon_minutes = $2 AND horizon_profile = 'uniform'
					       AND superseded_by IS NULL)
					 ORDER BY pr.ranking_status, pr.composite_score DESC NULLS LAST`,
					loc(i), horizons[i%len(horizons)]); err != nil {
					return err
				}
				return drain(ctx, pool,
					`SELECT temperature_c, precipitation_mm, observed_at, source, observation_type, quality_flag
					 FROM observations
					 WHERE location_id = $1 AND superseded_observation_id IS NULL AND quality_flag <> 'suspect'
					 ORDER BY observed_at DESC LIMIT 1`, loc(i))
			},
		},
		{
			id:   "Q-04",
			desc: "provider comparison grid (all locations × horizons, latest period)",
			run: func(ctx context.Context, pool *pgxpool.Pool, i int) error {
				return drain(ctx, pool,
					`SELECT pr.location_id, pr.horizon_minutes, pr.composite_score, pr.ranking_status,
					        pr.sample_count, pr.coverage, pr.reliability, pr.period_start, pr.period_end
					 FROM provider_rankings pr
					 WHERE pr.provider_id = $1 AND pr.superseded_by IS NULL
					   AND pr.period_end = (
					     SELECT max(period_end) FROM provider_rankings
					     WHERE provider_id = $1 AND superseded_by IS NULL)`,
					providers[i%len(providers)])
			},
		},
		{
			id:   "Q-05",
			desc: "metric trend across time (one cell, period range)",
			run: func(ctx context.Context, pool *pgxpool.Pool, i int) error {
				end := time.Now().UTC()
				return drain(ctx, pool,
					`SELECT value, ci_lower, ci_upper, sample_count, period_start, period_end
					 FROM accuracy_metrics
					 WHERE provider_id = $1 AND location_id = $2 AND horizon_minutes = $3
					   AND variable = 'temperature' AND metric_type = 'mae'
					   AND superseded_by IS NULL
					   AND period_start BETWEEN $4 AND $5
					 ORDER BY period_start`,
					providers[i%len(providers)], loc(i), horizons[i%len(horizons)],
					end.AddDate(0, 0, -365), end)
			},
		},
		{
			id:   "Q-09",
			desc: "forecast-vs-actual day payload (snapshots + observations)",
			run: func(ctx context.Context, pool *pgxpool.Pool, i int) error {
				// Rotate across the last 25 seeded days; partition pruning to
				// 1–2 monthly partitions is part of what this measures.
				dayStart := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -(1 + i%25))
				dayEnd := dayStart.Add(24 * time.Hour)
				if err := drain(ctx, pool,
					`SELECT id, provider_id, issued_at, target_time, forecast_horizon_minutes,
					        temperature_c, precipitation_probability, precipitation_amount_mm
					 FROM forecast_snapshots
					 WHERE location_id = $1 AND target_time BETWEEN $2 AND $3
					   AND forecast_horizon_minutes = $4 AND provider_id = ANY($5)`,
					loc(i), dayStart, dayEnd, horizons[i%len(horizons)], providers); err != nil {
					return err
				}
				return drain(ctx, pool,
					`SELECT id, observed_at, temperature_c, precipitation_mm, quality_flag
					 FROM observations
					 WHERE location_id = $1 AND observed_at BETWEEN $2 AND $3
					   AND superseded_observation_id IS NULL`,
					loc(i), dayStart, dayEnd)
			},
		},
	}

	fmt.Printf("=== PT-7 Query Baselines ===\n")
	fmt.Printf("Iterations: %d per pattern; gate p95 < %s (NFR-P08)\n\n", *iterations, *p95Gate)

	failed := false
	for _, p := range patterns {
		durations := make([]time.Duration, 0, *iterations)
		// One untimed warm-up to exclude plan/catalog cold start.
		if err := p.run(ctx, pool, 0); err != nil {
			return fmt.Errorf("%s warm-up: %w", p.id, err)
		}
		for i := 0; i < *iterations; i++ {
			t0 := time.Now()
			if err := p.run(ctx, pool, i); err != nil {
				return fmt.Errorf("%s iteration %d: %w", p.id, i, err)
			}
			durations = append(durations, time.Since(t0))
		}
		sort.Slice(durations, func(a, b int) bool { return durations[a] < durations[b] })
		q := func(pq float64) time.Duration { return durations[int(pq*float64(len(durations)-1))] }
		status := "PASS"
		if q(0.95) >= *p95Gate {
			status = "FAIL"
			failed = true
		}
		fmt.Printf("[%s] %-5s p50=%-8s p95=%-8s p99=%-8s max=%-8s  %s\n",
			status, p.id,
			q(0.50).Round(time.Microsecond), q(0.95).Round(time.Microsecond),
			q(0.99).Round(time.Microsecond), q(1.0).Round(time.Microsecond), p.desc)
	}
	if failed {
		return fmt.Errorf("p95 gate breached (NFR-P08); see PT-7 miss action in performance doc §5")
	}
	return nil
}
