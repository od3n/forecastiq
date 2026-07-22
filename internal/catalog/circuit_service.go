package catalog

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/catalog/domain"
	"github.com/forecastiq/forecastiq/internal/catalog/ports"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// CircuitService implements CircuitState (FC-09 breaker). State is persistent
// in provider_circuits; the record methods join the caller's (collection)
// transaction so the breaker update is atomic with the collection write.
type CircuitService struct {
	repo ports.CircuitRepository
	tx   *dbtx.Runner
}

// NewCircuitService wires a CircuitService.
func NewCircuitService(repo ports.CircuitRepository, tx *dbtx.Runner) *CircuitService {
	return &CircuitService{repo: repo, tx: tx}
}

// Evaluate decides whether a collection may proceed, persisting an
// open→half-open probe transition in its own short transaction.
func (s *CircuitService) Evaluate(ctx context.Context, providerID uuid.UUID, now time.Time) (domain.Decision, error) {
	var decision domain.Decision
	err := s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		circuit, err := s.repo.Get(ctx, tx, providerID)
		if err != nil {
			return err
		}
		before := circuit.State
		decision = circuit.Evaluate(now)
		if circuit.State != before {
			return s.repo.Upsert(ctx, tx, circuit)
		}
		return nil
	})
	return decision, err
}

// RecordSuccess closes the breaker within the caller's transaction.
func (s *CircuitService) RecordSuccess(ctx context.Context, tx dbtx.DBTX, providerID uuid.UUID, now time.Time) (Transition, error) {
	circuit, err := s.repo.Get(ctx, tx, providerID)
	if err != nil {
		return Transition{}, err
	}
	prev := circuit.ApplySuccess(now)
	if err := s.repo.Upsert(ctx, tx, circuit); err != nil {
		return Transition{}, err
	}
	return Transition{
		ProviderID:          providerID,
		Changed:             prev != circuit.State,
		Old:                 prev,
		New:                 circuit.State,
		ConsecutiveFailures: circuit.ConsecutiveFailures,
	}, nil
}

// RecordFailure advances the breaker toward open within the caller's tx.
func (s *CircuitService) RecordFailure(ctx context.Context, tx dbtx.DBTX, providerID uuid.UUID, now time.Time) (Transition, error) {
	circuit, err := s.repo.Get(ctx, tx, providerID)
	if err != nil {
		return Transition{}, err
	}
	prev := circuit.ApplyFailure(now)
	if err := s.repo.Upsert(ctx, tx, circuit); err != nil {
		return Transition{}, err
	}
	return Transition{
		ProviderID:          providerID,
		Changed:             prev != circuit.State,
		Old:                 prev,
		New:                 circuit.State,
		ConsecutiveFailures: circuit.ConsecutiveFailures,
	}, nil
}
