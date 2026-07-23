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

// AnalysisDispatcher dispatches analysis_batch slots to the matching engine.
// The batch is global (no location); it scans the whole unmatched window.
type AnalysisDispatcher struct {
	matcher BatchMatcher
	logger  *slog.Logger
}

// NewAnalysisDispatcher wires an AnalysisDispatcher.
func NewAnalysisDispatcher(matcher BatchMatcher, logger *slog.Logger) *AnalysisDispatcher {
	return &AnalysisDispatcher{matcher: matcher, logger: logger}
}

// Dispatch implements Dispatcher.
func (d *AnalysisDispatcher) Dispatch(ctx context.Context, slot *Slot) (int, error) {
	if slot.JobType != JobAnalysisBatch {
		return 0, fmt.Errorf("unsupported job type %q", slot.JobType)
	}
	return d.matcher.MatchBatch(ctx)
}
