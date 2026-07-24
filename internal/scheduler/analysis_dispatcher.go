package scheduler

import (
	"context"
	"fmt"
	"log/slog"
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
	logger     *slog.Logger
}

// NewAnalysisDispatcher wires an AnalysisDispatcher.
func NewAnalysisDispatcher(matcher BatchMatcher, aggregator BatchAggregator, ranker BatchRanker, logger *slog.Logger) *AnalysisDispatcher {
	return &AnalysisDispatcher{matcher: matcher, aggregator: aggregator, ranker: ranker, logger: logger}
}

// Dispatch implements Dispatcher: match → aggregate → rank. Returns the total
// of pairs created, metric rows written, and ranking rows published.
func (d *AnalysisDispatcher) Dispatch(ctx context.Context, slot *Slot) (int, error) {
	if slot.JobType != JobAnalysisBatch {
		return 0, fmt.Errorf("unsupported job type %q", slot.JobType)
	}
	pairs, err := d.matcher.MatchBatch(ctx)
	if err != nil {
		return pairs, err
	}
	rows, err := d.aggregator.AggregateBatch(ctx)
	if err != nil {
		return pairs + rows, err
	}
	rankings, err := d.ranker.RankBatch(ctx)
	if err != nil {
		return pairs + rows + rankings, err
	}
	return pairs + rows + rankings, nil
}
