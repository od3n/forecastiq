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

// ConfigurationRepository implements ports.ConfigurationRepository.
type ConfigurationRepository struct{}

// NewConfigurationRepository returns a ConfigurationRepository.
func NewConfigurationRepository() *ConfigurationRepository { return &ConfigurationRepository{} }

const configColumns = `id, workspace_id, provider_id, status, credential_ref,
	collection_schedule, adapter_version, validation_state, created_at, updated_at`

func scanConfiguration(row pgx.Row) (*domain.ProviderConfiguration, error) {
	var c domain.ProviderConfiguration
	var status string
	var credentialRef *string
	err := row.Scan(&c.ID, &c.WorkspaceID, &c.ProviderID, &status, &credentialRef,
		&c.CollectionSchedule, &c.AdapterVersion, &c.ValidationState, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("scan configuration: %w", err)
	}
	c.Status = domain.Status(status)
	if credentialRef != nil {
		c.CredentialRef = *credentialRef
	}
	return &c, nil
}

// GetByID implements ports.ConfigurationRepository.
func (r *ConfigurationRepository) GetByID(ctx context.Context, tx dbtx.DBTX, id uuid.UUID) (*domain.ProviderConfiguration, error) {
	return scanConfiguration(tx.QueryRow(ctx, `SELECT `+configColumns+` FROM provider_configurations WHERE id = $1`, id))
}

// GetByProviderID implements ports.ConfigurationRepository.
func (r *ConfigurationRepository) GetByProviderID(ctx context.Context, tx dbtx.DBTX, providerID uuid.UUID) (*domain.ProviderConfiguration, error) {
	return scanConfiguration(tx.QueryRow(ctx,
		`SELECT `+configColumns+` FROM provider_configurations WHERE provider_id = $1 ORDER BY created_at LIMIT 1`, providerID))
}

// ListActive implements ports.ConfigurationRepository.
func (r *ConfigurationRepository) ListActive(ctx context.Context, tx dbtx.DBTX) ([]*domain.ProviderConfiguration, error) {
	rows, err := tx.Query(ctx, `SELECT `+configColumns+` FROM provider_configurations WHERE status = 'active' ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list active configurations: %w", err)
	}
	defer rows.Close()
	var out []*domain.ProviderConfiguration
	for rows.Next() {
		c, err := scanConfiguration(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// List implements ports.ConfigurationRepository: all configurations regardless
// of status (admin surface — disabled configs must remain operable).
func (r *ConfigurationRepository) List(ctx context.Context, tx dbtx.DBTX) ([]*domain.ProviderConfiguration, error) {
	rows, err := tx.Query(ctx, `SELECT `+configColumns+` FROM provider_configurations ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list configurations: %w", err)
	}
	defer rows.Close()
	var out []*domain.ProviderConfiguration
	for rows.Next() {
		c, err := scanConfiguration(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Update implements ports.ConfigurationRepository: updates the operator-mutable
// fields (status, collection schedule, adapter version, validation state). The
// credential_ref is never mutated here (secret rotation is env-side; WP-18).
func (r *ConfigurationRepository) Update(ctx context.Context, tx dbtx.DBTX, c *domain.ProviderConfiguration) error {
	tag, err := tx.Exec(ctx,
		`UPDATE provider_configurations
		   SET status = $2, collection_schedule = $3, adapter_version = $4,
		       validation_state = $5, updated_at = $6
		 WHERE id = $1`,
		c.ID, string(c.Status), c.CollectionSchedule, c.AdapterVersion, c.ValidationState, c.UpdatedAt.UTC())
	if err != nil {
		return fmt.Errorf("update configuration: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Upsert implements ports.ConfigurationRepository (idempotent seed support).
func (r *ConfigurationRepository) Upsert(ctx context.Context, tx dbtx.DBTX, c *domain.ProviderConfiguration) error {
	credentialRef := any(nil)
	if c.CredentialRef != "" {
		credentialRef = c.CredentialRef
	}
	_, err := tx.Exec(ctx,
		`INSERT INTO provider_configurations (`+configColumns+`)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		 ON CONFLICT (id) DO UPDATE SET
		   status = EXCLUDED.status,
		   credential_ref = EXCLUDED.credential_ref,
		   collection_schedule = EXCLUDED.collection_schedule,
		   adapter_version = EXCLUDED.adapter_version,
		   validation_state = EXCLUDED.validation_state,
		   updated_at = EXCLUDED.updated_at`,
		c.ID, c.WorkspaceID, c.ProviderID, string(c.Status), credentialRef,
		c.CollectionSchedule, c.AdapterVersion, c.ValidationState, c.CreatedAt, c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert configuration: %w", err)
	}
	return nil
}
