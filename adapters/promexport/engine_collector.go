// Package promexport provides Prometheus collectors that compute gauge values
// at scrape time from authoritative external state (database, backup status
// file). Scrape-time reads keep the exported values truthful even when the
// producing process stops running — a plain in-process gauge would freeze at
// its last-set value and silently mask the exact failure the alert exists to
// detect (DRB-WP22-004). Collectors are wired in cmd/ (composition root).
package promexport

import (
	"context"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// queryTimeout bounds each scrape-time query so a slow database cannot stall
// the Prometheus scrape (scrape timeout is typically 10s).
const queryTimeout = 2 * time.Second

var (
	engineLagDesc = prometheus.NewDesc(
		"engine_lag_seconds",
		"Seconds since the last accuracy-metric batch completed (now − max calculated_at).",
		nil, nil)
	evaluationBacklogDesc = prometheus.NewDesc(
		"evaluation_backlog",
		"Matched pairs not yet aggregated into accuracy metrics.",
		nil, nil)
	rankingFreshnessDesc = prometheus.NewDesc(
		"ranking_freshness_age_seconds",
		"Age of the newest live ranking per location and horizon.",
		[]string{"location_id", "horizon_minutes"}, nil)
)

// EngineCollector exports the engine gauges of architecture §3.4 that are
// derived from database state: engine_lag_seconds, evaluation_backlog, and
// ranking_freshness_age_seconds. On query error the affected series are
// omitted from the scrape (absent metric) rather than reporting a stale or
// fabricated value; A7-style alerts must therefore pair with an absent() or
// up-based guard for full coverage.
type EngineCollector struct {
	pool *pgxpool.Pool
}

// NewEngineCollector returns a collector backed by the given pool.
func NewEngineCollector(pool *pgxpool.Pool) *EngineCollector {
	return &EngineCollector{pool: pool}
}

// Describe implements prometheus.Collector.
func (c *EngineCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- engineLagDesc
	ch <- evaluationBacklogDesc
	ch <- rankingFreshnessDesc
}

// Collect implements prometheus.Collector.
func (c *EngineCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	// engine_lag_seconds — absent until the first aggregation batch completes
	// (pre-launch there is legitimately no lag to report).
	var latest *time.Time
	if err := c.pool.QueryRow(ctx,
		`SELECT max(calculated_at) FROM accuracy_metrics WHERE superseded_by IS NULL`,
	).Scan(&latest); err == nil && latest != nil {
		lag := time.Since(*latest).Seconds()
		if lag < 0 {
			lag = 0
		}
		ch <- prometheus.MustNewConstMetric(engineLagDesc, prometheus.GaugeValue, lag)
	}

	// evaluation_backlog — matched pairs created after the newest live
	// accuracy metric (i.e. not yet covered by an aggregation batch).
	var backlog int64
	if err := c.pool.QueryRow(ctx,
		`SELECT count(*) FROM matched_evaluations
		  WHERE computed_at > COALESCE(
		    (SELECT max(calculated_at) FROM accuracy_metrics WHERE superseded_by IS NULL),
		    '-infinity')`,
	).Scan(&backlog); err == nil {
		ch <- prometheus.MustNewConstMetric(evaluationBacklogDesc, prometheus.GaugeValue, float64(backlog))
	}

	// ranking_freshness_age_seconds — one series per live (location, horizon).
	rows, err := c.pool.Query(ctx,
		`SELECT location_id::text, horizon_minutes,
		        extract(epoch FROM now() - max(calculated_at))
		   FROM provider_rankings
		  WHERE superseded_by IS NULL
		  GROUP BY location_id, horizon_minutes`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var locationID string
		var horizonMinutes int
		var age float64
		if err := rows.Scan(&locationID, &horizonMinutes, &age); err != nil {
			return
		}
		if age < 0 {
			age = 0
		}
		ch <- prometheus.MustNewConstMetric(rankingFreshnessDesc, prometheus.GaugeValue,
			age, locationID, strconv.Itoa(horizonMinutes))
	}
}
