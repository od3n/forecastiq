package metrics

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// poolCollector implements prometheus.Collector to expose pgxpool connection
// pool statistics at scrape time (observability architecture §3.6 Runtime).
type poolCollector struct {
	pool *pgxpool.Pool

	inUse *prometheus.Desc
	idle  *prometheus.Desc
	waits *prometheus.Desc
}

// newPoolCollector returns a collector wrapping the given pool.
func newPoolCollector(pool *pgxpool.Pool) *poolCollector {
	return &poolCollector{
		pool: pool,
		inUse: prometheus.NewDesc(
			"db_pool_in_use",
			"Number of connections currently acquired from the pool.",
			nil, nil,
		),
		idle: prometheus.NewDesc(
			"db_pool_idle",
			"Number of idle connections in the pool.",
			nil, nil,
		),
		waits: prometheus.NewDesc(
			"db_pool_wait_total",
			"Total number of acquire calls that had to wait for a connection.",
			nil, nil,
		),
	}
}

// Describe implements prometheus.Collector.
func (c *poolCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.inUse
	ch <- c.idle
	ch <- c.waits
}

// Collect implements prometheus.Collector. It reads pool stats at scrape time.
func (c *poolCollector) Collect(ch chan<- prometheus.Metric) {
	stat := c.pool.Stat()
	ch <- prometheus.MustNewConstMetric(c.inUse, prometheus.GaugeValue, float64(stat.AcquiredConns()))
	ch <- prometheus.MustNewConstMetric(c.idle, prometheus.GaugeValue, float64(stat.IdleConns()))
	ch <- prometheus.MustNewConstMetric(c.waits, prometheus.CounterValue, float64(stat.EmptyAcquireCount()))
}
