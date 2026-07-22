// Package catalog is the catalog module's application layer: it exposes the
// service interfaces other modules consume (LocationManager, ProviderCatalog,
// ConfigurationManager, CircuitState — module architecture §3.2) and
// implements them on top of the repository ports. Transaction boundaries:
// one tx per command (dedup check + insert + audit atomically).
package catalog

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/catalog/domain"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// Re-exported domain entity aliases so consuming modules reference catalog
// entities through the module's public surface (catalog.Provider, etc.).
type (
	Provider              = domain.Provider
	Location              = domain.Location
	ProviderConfiguration = domain.ProviderConfiguration
	ProviderCircuit       = domain.ProviderCircuit
	Status                = domain.Status
)

// Status values re-exported for consumer convenience.
const (
	StatusActive   = domain.StatusActive
	StatusDisabled = domain.StatusDisabled
	StatusArchived = domain.StatusArchived
)

// Actor identifies who performed an admin action (for audit). UserID is nil
// until WP-03; Name carries the dev-token actor label in the meantime.
type Actor struct {
	UserID    *uuid.UUID
	Name      string
	IPAddress string
}

// PageInfo is a keyset-pagination result.
type PageInfo struct {
	HasMore    bool
	NextCursor string
}

// CreateLocationInput is the CreateLocation command.
type CreateLocationInput struct {
	WorkspaceID        uuid.UUID
	Name               string
	Latitude           float64
	Longitude          float64
	CountryCode        string
	Timezone           string
	AllowNearDuplicate bool
	// OverrideReason documents why the operator accepted a near-duplicate
	// (audited when AllowNearDuplicate is set; WP-04 accountability).
	OverrideReason string
	Actor          Actor
}

// UpdateLocationInput is the UpdateLocation command. Only name is mutable
// (domain architecture §2.3: coordinates, country_code, and timezone are
// immutable after creation — a moved or re-zoned location is a new location).
type UpdateLocationInput struct {
	Name  *string
	Actor Actor
}

// ListLocationsInput is the location list query.
type ListLocationsInput struct {
	Active *bool
	Cursor string
	Limit  int
}

// LocationManager manages location lifecycle (BR-LOC-01..03).
type LocationManager interface {
	CreateLocation(ctx context.Context, in CreateLocationInput) (*domain.Location, error)
	GetLocation(ctx context.Context, id uuid.UUID) (*domain.Location, error)
	ListLocations(ctx context.Context, in ListLocationsInput) ([]*domain.Location, PageInfo, error)
	ListActiveLocations(ctx context.Context) ([]*domain.Location, error)
	UpdateLocation(ctx context.Context, id uuid.UUID, in UpdateLocationInput) (*domain.Location, error)
	SetLocationStatus(ctx context.Context, id uuid.UUID, status domain.Status, actor Actor) (*domain.Location, error)
}

// ProviderCatalog reads provider metadata + attribution.
type ProviderCatalog interface {
	GetProvider(ctx context.Context, id uuid.UUID) (*domain.Provider, error)
	GetProviderBySlug(ctx context.Context, slug string) (*domain.Provider, error)
	ListProviders(ctx context.Context) ([]*domain.Provider, error)
}

// ConfigurationManager reads provider operational configuration.
type ConfigurationManager interface {
	GetConfiguration(ctx context.Context, id uuid.UUID) (*domain.ProviderConfiguration, error)
	GetConfigurationByProviderID(ctx context.Context, providerID uuid.UUID) (*domain.ProviderConfiguration, error)
	ListActiveConfigurations(ctx context.Context) ([]*domain.ProviderConfiguration, error)
}

// Transition describes a circuit state change (drives provider.health_changed).
type Transition struct {
	ProviderID          uuid.UUID
	Changed             bool
	Old                 domain.CircuitState
	New                 domain.CircuitState
	ConsecutiveFailures int
}

// CircuitState is the breaker port consumed by the collection module. The
// record methods join the caller's transaction (collection tx); Evaluate runs
// its own short transaction as a pre-check.
type CircuitState interface {
	Evaluate(ctx context.Context, providerID uuid.UUID, now time.Time) (domain.Decision, error)
	RecordSuccess(ctx context.Context, tx dbtx.DBTX, providerID uuid.UUID, now time.Time) (Transition, error)
	RecordFailure(ctx context.Context, tx dbtx.DBTX, providerID uuid.UUID, now time.Time) (Transition, error)
}
