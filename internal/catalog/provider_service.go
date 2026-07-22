package catalog

import (
	"context"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/catalog/domain"
	"github.com/forecastiq/forecastiq/internal/catalog/ports"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// ProviderService implements ProviderCatalog (read-only at MVP; status
// changes land with the admin work package).
type ProviderService struct {
	repo ports.ProviderRepository
	pool dbtx.DBTX
}

// NewProviderService wires a ProviderService.
func NewProviderService(repo ports.ProviderRepository, pool dbtx.DBTX) *ProviderService {
	return &ProviderService{repo: repo, pool: pool}
}

// GetProvider returns a provider by id.
func (s *ProviderService) GetProvider(ctx context.Context, id uuid.UUID) (*domain.Provider, error) {
	return s.repo.GetByID(ctx, s.pool, id)
}

// GetProviderBySlug returns a provider by slug.
func (s *ProviderService) GetProviderBySlug(ctx context.Context, slug string) (*domain.Provider, error) {
	return s.repo.GetBySlug(ctx, s.pool, slug)
}

// ListProviders returns all providers (with attribution).
func (s *ProviderService) ListProviders(ctx context.Context) ([]*domain.Provider, error) {
	return s.repo.List(ctx, s.pool)
}

// ConfigurationService implements ConfigurationManager.
type ConfigurationService struct {
	repo ports.ConfigurationRepository
	pool dbtx.DBTX
}

// NewConfigurationService wires a ConfigurationService.
func NewConfigurationService(repo ports.ConfigurationRepository, pool dbtx.DBTX) *ConfigurationService {
	return &ConfigurationService{repo: repo, pool: pool}
}

// GetConfiguration returns a configuration by id.
func (s *ConfigurationService) GetConfiguration(ctx context.Context, id uuid.UUID) (*domain.ProviderConfiguration, error) {
	return s.repo.GetByID(ctx, s.pool, id)
}

// GetConfigurationByProviderID returns the operational config for a provider.
func (s *ConfigurationService) GetConfigurationByProviderID(ctx context.Context, providerID uuid.UUID) (*domain.ProviderConfiguration, error) {
	return s.repo.GetByProviderID(ctx, s.pool, providerID)
}

// ListActiveConfigurations returns configs the scheduler should generate
// slots for.
func (s *ConfigurationService) ListActiveConfigurations(ctx context.Context) ([]*domain.ProviderConfiguration, error) {
	return s.repo.ListActive(ctx, s.pool)
}
