package catalog

import (
	"context"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/audit"
	"github.com/forecastiq/forecastiq/internal/catalog/domain"
	"github.com/forecastiq/forecastiq/internal/catalog/ports"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// ProviderService implements ProviderCatalog (reads) and ProviderAdmin (the
// admin enable/disable mutation; WP-18). Mutations run in one bounded tx with
// their audit record (ADR-027).
type ProviderService struct {
	repo     ports.ProviderRepository
	pool     dbtx.DBTX
	tx       *dbtx.Runner
	recorder audit.Recorder
	clock    clock.Clock
}

// NewProviderService wires a ProviderService. tx/recorder/clock may be nil for
// read-only wiring (tests); they are required for the admin mutations.
func NewProviderService(repo ports.ProviderRepository, pool dbtx.DBTX,
	tx *dbtx.Runner, recorder audit.Recorder, clk clock.Clock) *ProviderService {
	return &ProviderService{repo: repo, pool: pool, tx: tx, recorder: recorder, clock: clk}
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

// SetProviderStatus enables or disables a provider (S-11). Only active|disabled
// are settable (archived reserved, mirroring locations). Audited.
func (s *ProviderService) SetProviderStatus(ctx context.Context, id uuid.UUID, status domain.Status, actor Actor) (*domain.Provider, error) {
	if !status.Settable() {
		ve := &domain.ValidationError{}
		ve.Add("status", "must be one of active|disabled")
		return nil, ve
	}
	var updated *domain.Provider
	err := s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		if _, gerr := s.repo.GetByID(ctx, tx, id); gerr != nil {
			return gerr
		}
		now := s.clock.Now()
		if serr := s.repo.SetStatus(ctx, tx, id, status, now); serr != nil {
			return serr
		}
		p, gerr := s.repo.GetByID(ctx, tx, id)
		if gerr != nil {
			return gerr
		}
		updated = p
		return s.recorder.Record(ctx, tx, audit.Event{
			UserID: actor.UserID, Action: "provider.set_status", ResourceType: "provider",
			ResourceID: &id, IPAddress: actor.IPAddress,
			Details: map[string]any{"status": string(status), "actor": actor.Name}, At: now,
		})
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// ConfigurationService implements ConfigurationManager (reads) and
// ConfigurationAdmin (the operator config update; WP-18).
type ConfigurationService struct {
	repo     ports.ConfigurationRepository
	pool     dbtx.DBTX
	tx       *dbtx.Runner
	recorder audit.Recorder
	clock    clock.Clock
}

// NewConfigurationService wires a ConfigurationService.
func NewConfigurationService(repo ports.ConfigurationRepository, pool dbtx.DBTX,
	tx *dbtx.Runner, recorder audit.Recorder, clk clock.Clock) *ConfigurationService {
	return &ConfigurationService{repo: repo, pool: pool, tx: tx, recorder: recorder, clock: clk}
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

// ListConfigurations returns all configurations (active + disabled) for the
// admin surface (S-11).
func (s *ConfigurationService) ListConfigurations(ctx context.Context) ([]*domain.ProviderConfiguration, error) {
	return s.repo.List(ctx, s.pool)
}

// UpdateConfigInput carries the operator-mutable configuration fields; nil
// fields are left unchanged. The credential reference is never mutated via the
// API (secret rotation is env-side, BR-08).
type UpdateConfigInput struct {
	Status          *domain.Status
	MinuteOffset    *int
	AdapterVersion  *string
	ValidationState *string
}

// UpdateConfiguration applies the operator-mutable config fields (S-11) and
// audits the change. Returns a validation error for an unsettable status or an
// out-of-range minute offset.
func (s *ConfigurationService) UpdateConfiguration(ctx context.Context, id uuid.UUID, in UpdateConfigInput, actor Actor) (*domain.ProviderConfiguration, error) {
	ve := &domain.ValidationError{}
	if in.Status != nil && !in.Status.Settable() {
		ve.Add("status", "must be one of active|disabled")
	}
	if in.MinuteOffset != nil && (*in.MinuteOffset < 0 || *in.MinuteOffset > 59) {
		ve.Add("minute_offset", "must be between 0 and 59")
	}
	if err := ve.ErrorOrNil(); err != nil {
		return nil, err
	}

	var updated *domain.ProviderConfiguration
	fields := make([]string, 0, 4)
	err := s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		c, gerr := s.repo.GetByID(ctx, tx, id)
		if gerr != nil {
			return gerr
		}
		if in.Status != nil {
			c.Status = *in.Status
			fields = append(fields, "status")
		}
		if in.MinuteOffset != nil {
			c.CollectionSchedule.MinuteOffset = *in.MinuteOffset
			fields = append(fields, "minute_offset")
		}
		if in.AdapterVersion != nil {
			c.AdapterVersion = *in.AdapterVersion
			fields = append(fields, "adapter_version")
		}
		if in.ValidationState != nil {
			c.ValidationState = *in.ValidationState
			fields = append(fields, "validation_state")
		}
		c.UpdatedAt = s.clock.Now()
		if uerr := s.repo.Update(ctx, tx, c); uerr != nil {
			return uerr
		}
		updated = c
		return s.recorder.Record(ctx, tx, audit.Event{
			UserID: actor.UserID, Action: "provider_configuration.update", ResourceType: "provider_configuration",
			ResourceID: &id, IPAddress: actor.IPAddress,
			Details: map[string]any{"fields": fields, "actor": actor.Name}, At: s.clock.Now(),
		})
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}
