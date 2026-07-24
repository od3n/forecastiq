package admin

import (
	"context"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/audit"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// BatchRunner runs the analysis pipeline (match → aggregate → rank) on demand,
// returning records affected. Implemented by scheduler.AnalysisDispatcher.
type BatchRunner interface {
	Recompute(ctx context.Context) (int, error)
}

// RecomputeActor identifies the operator who triggered a recompute (for audit).
type RecomputeActor struct {
	UserID    *uuid.UUID
	Name      string
	IPAddress string
}

// RecomputeService runs an on-demand analysis recompute (S-13 admin) and audits
// the trigger. The batch's own writes are transactional per cell; the audit
// event records who triggered it and how many rows were affected.
type RecomputeService struct {
	runner   BatchRunner
	tx       *dbtx.Runner
	recorder audit.Recorder
	clock    clock.Clock
}

// NewRecomputeService wires a RecomputeService.
func NewRecomputeService(runner BatchRunner, tx *dbtx.Runner, recorder audit.Recorder, clk clock.Clock) *RecomputeService {
	return &RecomputeService{runner: runner, tx: tx, recorder: recorder, clock: clk}
}

// Recompute runs the pipeline and records an audit event. Returns the combined
// records-affected count.
func (s *RecomputeService) Recompute(ctx context.Context, actor RecomputeActor) (int, error) {
	affected, err := s.runner.Recompute(ctx)
	if err != nil {
		return affected, err
	}
	now := s.clock.Now()
	if aerr := s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		return s.recorder.Record(ctx, tx, audit.Event{
			UserID: actor.UserID, Action: "analysis.recompute", ResourceType: "analysis",
			IPAddress: actor.IPAddress,
			Details:   map[string]any{"records_affected": affected, "actor": actor.Name}, At: now,
		})
	}); aerr != nil {
		return affected, aerr
	}
	return affected, nil
}
