// Package ports declares the catalog module's repository contracts. The
// persistence adapters (adapters/persistence/catalogpg) implement these; the
// application services depend only on the interfaces (dependency rule:
// dependencies point inward). Every method accepts a dbtx.DBTX so it can run
// against the pool or inside a caller-owned transaction.
package ports

import (
	"context"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/catalog/domain"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// LocationFilter selects locations for listing. Cursor is the last-seen id
// (exclusive) for keyset pagination; empty starts from the beginning.
type LocationFilter struct {
	Active *bool
	Cursor uuid.UUID
	Limit  int
}

// LocationRepository persists Location aggregates.
type LocationRepository interface {
	// AcquireDedupLock serializes the BR-LOC-01 proximity-check window within
	// the caller's transaction (pg_advisory_xact_lock). Prevents concurrent
	// creates from both passing the haversine check before either commits.
	AcquireDedupLock(ctx context.Context, tx dbtx.DBTX) error
	Insert(ctx context.Context, tx dbtx.DBTX, l *domain.Location) error
	GetByID(ctx context.Context, tx dbtx.DBTX, id uuid.UUID) (*domain.Location, error)
	// List returns up to filter.Limit+1 rows (the extra detects has_more).
	List(ctx context.Context, tx dbtx.DBTX, f LocationFilter) ([]*domain.Location, error)
	ListActive(ctx context.Context, tx dbtx.DBTX) ([]*domain.Location, error)
	Update(ctx context.Context, tx dbtx.DBTX, l *domain.Location) error
	UpdateStatus(ctx context.Context, tx dbtx.DBTX, id uuid.UUID, status domain.Status) error
}

// ProviderRepository persists Provider aggregates.
type ProviderRepository interface {
	GetByID(ctx context.Context, tx dbtx.DBTX, id uuid.UUID) (*domain.Provider, error)
	GetBySlug(ctx context.Context, tx dbtx.DBTX, slug string) (*domain.Provider, error)
	List(ctx context.Context, tx dbtx.DBTX) ([]*domain.Provider, error)
	Upsert(ctx context.Context, tx dbtx.DBTX, p *domain.Provider) error
}

// ConfigurationRepository persists ProviderConfiguration aggregates.
type ConfigurationRepository interface {
	GetByID(ctx context.Context, tx dbtx.DBTX, id uuid.UUID) (*domain.ProviderConfiguration, error)
	GetByProviderID(ctx context.Context, tx dbtx.DBTX, providerID uuid.UUID) (*domain.ProviderConfiguration, error)
	ListActive(ctx context.Context, tx dbtx.DBTX) ([]*domain.ProviderConfiguration, error)
	Upsert(ctx context.Context, tx dbtx.DBTX, c *domain.ProviderConfiguration) error
}

// CircuitRepository persists provider circuit-breaker state.
type CircuitRepository interface {
	// Get returns the circuit for a provider, or a fresh closed circuit when
	// no row exists yet.
	Get(ctx context.Context, tx dbtx.DBTX, providerID uuid.UUID) (*domain.ProviderCircuit, error)
	Upsert(ctx context.Context, tx dbtx.DBTX, c *domain.ProviderCircuit) error
}
