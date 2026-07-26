package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/forecastiq/forecastiq/internal/platform/metrics"
)

// BatchMatcher runs one matching batch, returning pairs created. Implemented by
// analysis.MatchService.
type BatchMatcher interface {
	MatchBatch(ctx context.Context) (int, error)
}

// BatchAggregator recomputes accuracy metrics for the rolling period set,
// returning rows written. Implemented by analysis.AggregateService.
type BatchAggregator interface {
	AggregateBatch(ctx context.Context) (int, error)
}

// BatchRanker recomputes provider rankings for the current period, returning
// rows published. Implemented by analysis.RankService.
type BatchRanker interface {
	RankBatch(ctx context.Context) (int, error)
}

// AnalysisDispatcher dispatches analysis_batch slots to the analysis pipeline:
// matching, then aggregation, then ranking (workflow §1, sequential within the
// batch). The batch is global (no location).
type AnalysisDispatcher struct {
	matcher    BatchMatcher
	aggregator BatchAggregator
	ranker     BatchRanker
	metrics    *metrics.Metrics
	logger     *slog.Logger
}

// NewAnalysisDispatcher wires an AnalysisDispatcher.
func NewAnalysisDispatcher(matcher BatchMatcher, aggregator BatchAggregator, ranker BatchRanker, m *metrics.Metrics, logger *slog.Logger) *AnalysisDispatcher {
	return &AnalysisDispatcher{matcher: matcher, aggregator: aggregator, ranker: ranker, metrics: m, logger: logger}
}

// Dispatch implements Dispatcher: match → aggregate → rank. Returns the total
// of pairs created, metric rows written, and ranking rows published.
func (d *AnalysisDispatcher) Dispatch(ctx context.Context, slot *Slot) (int, error) {
	if slot.JobType != JobAnalysisBatch {
		return 0, fmt.Errorf("unsupported job type %q", slot.JobType)
	}
	return d.run(ctx)
}

// Recompute runs the analysis pipeline on demand (admin action; S-13), outside
// the scheduler. Same match → aggregate → rank sequence as a scheduled batch.
func (d *AnalysisDispatcher) Recompute(ctx context.Context) (int, error) {
	return d.run(ctx)
}

// run executes match → aggregate → rank sequentially, recording per-step
// durations on batch_duration_seconds and updating engine lag after completion.
func (d *AnalysisDispatcher) run(ctx context.Context) (int, error) {
	var total int

	// Matching
	start := time.Now()
	pairs, err := d.matcher.MatchBatch(ctx)
	d.metrics.BatchDuration.WithLabelValues("matching").Observe(time.Since(start).Seconds())
	total += pairs
	if err != nil {
		return total, err
	}

	// Aggregation
	start = time.Now()
	rows, err := d.aggregator.AggregateBatch(ctx)
	d.metrics.BatchDuration.WithLabelValues("aggregation").Observe(time.Since(start).Seconds())
	total += rows
	if err != nil {
		return total, err
	}

	// Ranking
	start = time.Now()
	rankings, err := d.ranker.RankBatch(ctx)
	d.metrics.BatchDuration.WithLabelValues("ranking").Observe(time.Since(start).Seconds())
	total += rankings
	if err != nil {
		return total, err
	}

	// Engine lag / backlog / ranking freshness are exported at scrape time by
	// the DB-backed collector (adapters/promexport), so a stalled batch is
	// visible to alerting without any in-process bookkeeping here.
	return total, nil
}
