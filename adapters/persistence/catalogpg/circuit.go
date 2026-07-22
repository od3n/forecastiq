package catalogpg

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/forecastiq/forecastiq/internal/catalog/domain"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// CircuitRepository implements ports.CircuitRepository (persistent breaker
// state; FC-09).
type CircuitRepository struct{}

// NewCircuitRepository returns a CircuitRepository.
func NewCircuitRepository() *CircuitRepository { return &CircuitRepository{} }

// Get implements ports.CircuitRepository. A missing row yields a fresh closed
// circuit (first collection for a provider).
func (r *CircuitRepository) Get(ctx context.Context, tx dbtx.DBTX, providerID uuid.UUID) (*domain.ProviderCircuit, error) {
	var c domain.ProviderCircuit
	var state string
	err := tx.QueryRow(ctx,
		`SELECT provider_id, state, consecutive_failures, last_failure_at, opened_at, next_probe_at, updated_at
		 FROM provider_circuits WHERE provider_id = $1`, providerID).
		Scan(&c.ProviderID, &state, &c.ConsecutiveFailures, &c.LastFailureAt, &c.OpenedAt, &c.NextProbeAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.NewProviderCircuit(providerID), nil
		}
		return nil, fmt.Errorf("scan circuit: %w", err)
	}
	c.State = domain.CircuitState(state)
	return &c, nil
}

// Upsert implements ports.CircuitRepository.
func (r *CircuitRepository) Upsert(ctx context.Context, tx dbtx.DBTX, c *domain.ProviderCircuit) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO provider_circuits
		   (provider_id, state, consecutive_failures, last_failure_at, opened_at, next_probe_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 ON CONFLICT (provider_id) DO UPDATE SET
		   state = EXCLUDED.state,
		   consecutive_failures = EXCLUDED.consecutive_failures,
		   last_failure_at = EXCLUDED.last_failure_at,
		   opened_at = EXCLUDED.opened_at,
		   next_probe_at = EXCLUDED.next_probe_at,
		   updated_at = EXCLUDED.updated_at`,
		c.ProviderID, string(c.State), c.ConsecutiveFailures, c.LastFailureAt, c.OpenedAt, c.NextProbeAt, c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert circuit: %w", err)
	}
	return nil
}
