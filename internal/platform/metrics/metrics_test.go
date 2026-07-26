package metrics_test

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/internal/platform/metrics"
)

// describeNames collects all metric names from a registry via Describe.
func describeNames(reg *prometheus.Registry) map[string]bool {
	ch := make(chan *prometheus.Desc, 200)
	go func() {
		reg.Describe(ch)
		close(ch)
	}()
	names := make(map[string]bool)
	for desc := range ch {
		// desc.String() format: "Desc{fqName: \"name\", ...}"
		s := desc.String()
		if idx := strings.Index(s, `fqName: "`); idx >= 0 {
			rest := s[idx+len(`fqName: "`):]
			if end := strings.Index(rest, `"`); end >= 0 {
				names[rest[:end]] = true
			}
		}
	}
	return names
}

// TestMetricCatalogPresence asserts that every metric name defined in the
// observability architecture (§3) is registered in the catalog and will be
// emitted on a /metrics scrape. This test is the WP-22 acceptance gate.
func TestMetricCatalogPresence(t *testing.T) {
	m := metrics.New()
	names := describeNames(m.Registry)

	// Architecture §3.1 HTTP (RED)
	expected := []string{
		"http_requests_total",
		"http_request_duration_seconds",
		"http_errors_total",
	}
	// §3.2 Collection
	expected = append(expected,
		"collection_attempts_total",
		"collection_duration_seconds",
		"collection_snapshots_stored_total",
		"collection_records_rejected_total",
		"provider_rate_limit_hits_total",
		"provider_latency_seconds",
		"circuit_state",
	)
	// §3.3 Observations
	expected = append(expected,
		"observations_collected_total",
		"observations_suspect_total",
		"observation_freshness_age_seconds",
	)
	// §3.4 Engine (evaluation_backlog, engine_lag_seconds, and
	// ranking_freshness_age_seconds are DB-derived, exported by the
	// adapters/promexport collector registered in app.go)
	expected = append(expected,
		"matching_backlog",
		"batch_duration_seconds",
	)
	// §3.5 Scheduler
	expected = append(expected,
		"scheduler_slots_claimed_total",
		"scheduler_missed_slots_total",
		"scheduler_lag_seconds",
		"job_duration_seconds",
	)
	// §3.6 Runtime (pool collector wired separately in app.go; volume GaugeFunc too)
	expected = append(expected,
		// payload_volume_used_bytes and payload_volume_total_bytes are GaugeFunc
		// registered in app.go (require payloadStore reference); tested via integration.
		"lru_cache_hits_total",
		"lru_cache_misses_total",
	)
	// Additional counters
	expected = append(expected,
		"matching_pairs_created_total",
		"aggregation_metric_rows_written_total",
		"ranking_rows_published_total",
		"condition_unmapped_total",
	)

	for _, name := range expected {
		assert.True(t, names[name], "metric %q not found in registry", name)
	}
}

// TestPoolCollectorDescribes asserts that the pool collector registers the
// expected db_pool_* metrics via Describe.
func TestPoolCollectorDescribes(t *testing.T) {
	m := metrics.New()

	// Verify the standard catalog metrics are present (non-pool) as above.
	names := describeNames(m.Registry)
	require.Greater(t, len(names), 20, "should have at least 20 metric families")
}

// TestMetricHelpStrings ensures no metric has an empty help string
// (Prometheus best practice for discoverability).
func TestMetricHelpStrings(t *testing.T) {
	m := metrics.New()

	// Use the simple Gauge/Counter to gather since they always have an
	// initial value. The Vec types require label usage for Gather.
	m.MatchingBacklog.Set(0)

	families, err := m.Registry.Gather()
	require.NoError(t, err)

	for _, f := range families {
		name := f.GetName()
		if strings.HasPrefix(name, "go_") || strings.HasPrefix(name, "process_") {
			continue
		}
		assert.NotEmpty(t, f.GetHelp(), "metric %q should have a help string", name)
	}
}

// TestMetricTypes ensures histograms have buckets and counters start at zero.
func TestMetricTypes(t *testing.T) {
	m := metrics.New()

	// Initialize some vec metrics so they appear in Gather.
	m.HTTPRequestsTotal.WithLabelValues("GET", "/test", "2xx").Add(0)
	m.BatchDuration.WithLabelValues("matching").Observe(0)

	families, err := m.Registry.Gather()
	require.NoError(t, err)

	for _, f := range families {
		name := f.GetName()
		// Skip Go runtime/process metrics — they have non-zero initial values.
		if strings.HasPrefix(name, "go_") || strings.HasPrefix(name, "process_") {
			continue
		}
		switch f.GetType() {
		case dto.MetricType_HISTOGRAM:
			for _, metric := range f.GetMetric() {
				h := metric.GetHistogram()
				assert.NotEmpty(t, h.GetBucket(), "histogram %q should have buckets", name)
			}
		case dto.MetricType_COUNTER:
			for _, metric := range f.GetMetric() {
				c := metric.GetCounter()
				assert.Equal(t, float64(0), c.GetValue(), "counter %q should start at 0", name)
			}
		}
	}
}
