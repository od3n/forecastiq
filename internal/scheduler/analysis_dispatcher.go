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

// AnalysisDispatcher dispatches analysis_batch slots to the analysis pipeline:
// matching first, then aggregation over the freshly matched pairs (workflow §1,
// sequential within the batch). The batch is global (no location).
type AnalysisDispatcher struct {
	matcher    BatchMatcher
	aggregator BatchAggregator
	logger     *slog.Logger
}

// NewAnalysisDispatcher wires an AnalysisDispatcher.
func NewAnalysisDispatcher(matcher BatchMatcher, aggregator BatchAggregator, logger *slog.Logger) *AnalysisDispatcher {
	return &AnalysisDispatcher{matcher: matcher, aggregator: aggregator, logger: logger}
}

// Dispatch implements Dispatcher: match, then aggregate. Returns pairs created
// plus metric rows written.
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
	return pairs + rows, nil
}
