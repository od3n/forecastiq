// Package metrics owns the Prometheus registry and the metric catalog
// (observability architecture §3). Metrics are registered against a
// dedicated registry (no global state) so tests stay isolated. The slice
// instruments HTTP RED, collection, provider, circuit, condition-mapping,
// and scheduler metrics; later work packages extend the catalog.
package metrics

import (
	"net/http"

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

	// Scheduler
	SlotsClaimed *prometheus.CounterVec
	MissedSlots  *prometheus.CounterVec
	SchedulerLag *prometheus.HistogramVec
	JobDuration  *prometheus.HistogramVec
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

	reg.MustRegister(
		m.HTTPRequestsTotal, m.HTTPRequestDuration, m.HTTPErrorsTotal,
		m.CollectionAttempts, m.CollectionDuration, m.SnapshotsStored, m.RecordsRejected,
		m.RateLimitHits, m.ProviderLatency, m.CircuitState,
		m.ConditionUnmapped,
		m.SlotsClaimed, m.MissedSlots, m.SchedulerLag, m.JobDuration,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return m
}

// Handler returns the Prometheus exposition HTTP handler.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{})
}
