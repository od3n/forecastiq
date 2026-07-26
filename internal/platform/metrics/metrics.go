// Package metrics owns the Prometheus registry and the metric catalog
// (observability architecture §3). Metrics are registered against a
// dedicated registry (no global state) so tests stay isolated. The full
// catalog covers HTTP RED, collection, provider, circuit, condition-mapping,
// observation, analysis engine, scheduler, and runtime metrics (WP-22).
package metrics

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all registered collectors.
type Metrics struct {
	Registry *prometheus.Registry

	// HTTP (RED)
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec
	HTTPErrorsTotal     *prometheus.CounterVec

	// Collection
	CollectionAttempts *prometheus.CounterVec
	CollectionDuration *prometheus.HistogramVec
	SnapshotsStored    *prometheus.CounterVec
	RecordsRejected    *prometheus.CounterVec

	// Provider
	RateLimitHits   *prometheus.CounterVec
	ProviderLatency *prometheus.HistogramVec
	CircuitState    *prometheus.GaugeVec

	// Condition taxonomy
	ConditionUnmapped *prometheus.CounterVec

	// Observation collection (WP-10)
	ObservationsCollected *prometheus.CounterVec
	ObservationsSuspect   *prometheus.CounterVec
	ObservationFreshness  *prometheus.GaugeVec

	// Analysis / matching (WP-11)
	MatchesCreated  prometheus.Counter
	MatchingBacklog prometheus.Gauge

	// Analysis / aggregation (WP-13)
	MetricRowsWritten prometheus.Counter

	// Analysis / ranking (WP-14)
	RankingsPublished prometheus.Counter

	// HTTP read-path LRU cache (WP-15)
	CacheHits   *prometheus.CounterVec
	CacheMisses *prometheus.CounterVec

	// Scheduler
	SlotsClaimed *prometheus.CounterVec
	MissedSlots  *prometheus.CounterVec
	SchedulerLag *prometheus.HistogramVec
	JobDuration  *prometheus.HistogramVec

	// Engine (WP-22, architecture §3.4)
	EvaluationBacklog prometheus.Gauge
	EngineLag         prometheus.Gauge
	RankingFreshness  *prometheus.GaugeVec
	BatchDuration     *prometheus.HistogramVec

	// Runtime (WP-22, architecture §3.6)
	// Note: payload_volume_used_bytes and payload_volume_total_bytes are
	// registered as GaugeFunc in the composition root (app.go) since they
	// require the payloadStore reference for scrape-time reads.
}

// New builds and registers the metric catalog on a fresh registry.
func New() *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{Registry: reg}

	m.HTTPRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests.",
	}, []string{"method", "route_template", "status_class"})

	m.HTTPRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency.",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.2, 0.5, 1, 2, 5},
	}, []string{"method", "route_template"})

	m.HTTPErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_errors_total",
		Help: "Total HTTP errors by envelope class.",
	}, []string{"route_template", "error_type"})

	m.CollectionAttempts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "collection_attempts_total",
		Help: "Forecast collection attempts by provider and terminal status.",
	}, []string{"provider", "status"})

	m.CollectionDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "collection_duration_seconds",
		Help:    "End-to-end forecast collection duration.",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 20},
	}, []string{"provider"})

	m.SnapshotsStored = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "collection_snapshots_stored_total",
		Help: "Forecast snapshots persisted.",
	}, []string{"provider"})

	m.RecordsRejected = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "collection_records_rejected_total",
		Help: "Provider records rejected during validation.",
	}, []string{"provider", "reason"})

	m.RateLimitHits = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "provider_rate_limit_hits_total",
		Help: "Provider rate-limit (429 / budget) hits.",
	}, []string{"provider"})

	m.ProviderLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "provider_latency_seconds",
		Help:    "Upstream provider HTTP latency.",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10},
	}, []string{"provider"})

	m.CircuitState = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "circuit_state",
		Help: "Provider circuit state (0=closed, 1=half_open, 2=open).",
	}, []string{"provider"})

	m.ConditionUnmapped = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "condition_unmapped_total",
		Help: "Provider condition codes with no canonical mapping.",
	}, []string{"provider", "code"})

	m.ObservationsCollected = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "observations_collected_total",
		Help: "Observation rows stored by source and location.",
	}, []string{"source", "location_id"})

	m.ObservationsSuspect = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "observations_suspect_total",
		Help: "Observation rows flagged suspect (OC-04 range violations).",
	}, []string{"source", "reason"})

	m.ObservationFreshness = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "observation_freshness_age_seconds",
		Help: "Age of the newest observation per location (BR-FRESH).",
	}, []string{"location_id"})

	m.MatchesCreated = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "matching_pairs_created_total",
		Help: "Snapshot–observation pairs created by the matching engine (incl. rematch).",
	})

	m.MatchingBacklog = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "matching_backlog",
		Help: "Unmatched forecast snapshots within the matching window.",
	})

	m.MetricRowsWritten = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "aggregation_metric_rows_written_total",
		Help: "AccuracyMetric rows written by the aggregation batch (incl. recompute).",
	})

	m.RankingsPublished = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ranking_rows_published_total",
		Help: "ProviderRanking rows published by the ranking batch (incl. recompute).",
	})

	m.CacheHits = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "lru_cache_hits_total",
		Help: "Read-path LRU cache hits (fresh entry served or 304).",
	}, []string{"route_template"})

	m.CacheMisses = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "lru_cache_misses_total",
		Help: "Read-path LRU cache misses (handler executed).",
	}, []string{"route_template"})

	m.SlotsClaimed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "scheduler_slots_claimed_total",
		Help: "Scheduler slots claimed.",
	}, []string{"job_type"})

	m.MissedSlots = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "scheduler_missed_slots_total",
		Help: "Scheduler slots that became due without being claimed in time.",
	}, []string{"job_type"})

	m.SchedulerLag = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "scheduler_lag_seconds",
		Help:    "Delay between a slot's scheduled time and when it was claimed.",
		Buckets: []float64{1, 5, 15, 30, 60, 120, 300, 900, 3600},
	}, []string{"job_type"})

	m.JobDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "job_duration_seconds",
		Help:    "Worker job execution duration.",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60},
	}, []string{"job_type"})

	// Engine metrics (architecture §3.4)
	m.EvaluationBacklog = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "evaluation_backlog",
		Help: "Matched pairs not yet aggregated into accuracy metrics.",
	})

	m.EngineLag = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "engine_lag_seconds",
		Help: "Seconds since the last accuracy-metric batch completed (now − max calculated_at).",
	})

	m.RankingFreshness = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ranking_freshness_age_seconds",
		Help: "Age of the newest ranking per location and horizon.",
	}, []string{"location_id", "horizon_minutes"})

	m.BatchDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "batch_duration_seconds",
		Help:    "Duration of individual analysis batch sub-steps.",
		Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120, 300, 600},
	}, []string{"batch_type"})

	// Runtime metrics (architecture §3.6)
	// payload_volume_*_bytes registered as GaugeFunc in app.go (scrape-time reads).

	reg.MustRegister(
		m.HTTPRequestsTotal, m.HTTPRequestDuration, m.HTTPErrorsTotal,
		m.CollectionAttempts, m.CollectionDuration, m.SnapshotsStored, m.RecordsRejected,
		m.RateLimitHits, m.ProviderLatency, m.CircuitState,
		m.ConditionUnmapped,
		m.ObservationsCollected, m.ObservationsSuspect, m.ObservationFreshness,
		m.MatchesCreated, m.MatchingBacklog, m.MetricRowsWritten, m.RankingsPublished,
		m.CacheHits, m.CacheMisses,
		m.SlotsClaimed, m.MissedSlots, m.SchedulerLag, m.JobDuration,
		m.EvaluationBacklog, m.EngineLag, m.RankingFreshness, m.BatchDuration,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return m
}

// RegisterPoolCollector registers a custom collector that exposes pgxpool
// connection-pool statistics as Prometheus metrics at scrape time.
func (m *Metrics) RegisterPoolCollector(pool *pgxpool.Pool) {
	m.Registry.MustRegister(newPoolCollector(pool))
}

// Handler returns the Prometheus exposition HTTP handler.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{})
}
