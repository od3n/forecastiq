package catalogpg

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/forecastiq/forecastiq/internal/catalog/domain"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// ProviderRepository implements ports.ProviderRepository.
type ProviderRepository struct{}

// NewProviderRepository returns a ProviderRepository.
func NewProviderRepository() *ProviderRepository { return &ProviderRepository{} }

const providerColumns = `id, name, slug, api_base_url, status, attribution_text, attribution_url, created_at, updated_at`

func scanProvider(row pgx.Row) (*domain.Provider, error) {
	var p domain.Provider
	var status string
	err := row.Scan(&p.ID, &p.Name, &p.Slug, &p.APIBaseURL, &status,
		&p.AttributionText, &p.AttributionURL, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("scan provider: %w", err)
	}
	p.Status = domain.Status(status)
	return &p, nil
}

// GetByID implements ports.ProviderRepository.
func (r *ProviderRepository) GetByID(ctx context.Context, tx dbtx.DBTX, id uuid.UUID) (*domain.Provider, error) {
	return scanProvider(tx.QueryRow(ctx, `SELECT `+providerColumns+` FROM providers WHERE id = $1`, id))
}

// GetBySlug implements ports.ProviderRepository.
func (r *ProviderRepository) GetBySlug(ctx context.Context, tx dbtx.DBTX, slug string) (*domain.Provider, error) {
	return scanProvider(tx.QueryRow(ctx, `SELECT `+providerColumns+` FROM providers WHERE slug = $1`, slug))
}

// List implements ports.ProviderRepository.
func (r *ProviderRepository) List(ctx context.Context, tx dbtx.DBTX) ([]*domain.Provider, error) {
	rows, err := tx.Query(ctx, `SELECT `+providerColumns+` FROM providers ORDER BY slug`)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	defer rows.Close()
	var out []*domain.Provider
	for rows.Next() {
		var p domain.Provider
		var status string
		if err := rows.Scan(&p.ID, &p.Name, &p.Slug, &p.APIBaseURL, &status,
			&p.AttributionText, &p.AttributionURL, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan provider row: %w", err)
		}
		p.Status = domain.Status(status)
		out = append(out, &p)
	}
	return out, rows.Err()
}

// SetStatus implements ports.ProviderRepository: updates the lifecycle status
// (admin enable/disable; WP-18). Returns domain.ErrNotFound when no row matches.
func (r *ProviderRepository) SetStatus(ctx context.Context, tx dbtx.DBTX, id uuid.UUID, status domain.Status, now time.Time) error {
	tag, err := tx.Exec(ctx, `UPDATE providers SET status = $2, updated_at = $3 WHERE id = $1`,
		id, string(status), now.UTC())
	if err != nil {
		return fmt.Errorf("set provider status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Upsert implements ports.ProviderRepository (idempotent seed support).
func (r *ProviderRepository) Upsert(ctx context.Context, tx dbtx.DBTX, p *domain.Provider) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO providers (`+providerColumns+`)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 ON CONFLICT (id) DO UPDATE SET
		   name = EXCLUDED.name,
		   api_base_url = EXCLUDED.api_base_url,
		   status = EXCLUDED.status,
		   attribution_text = EXCLUDED.attribution_text,
		   attribution_url = EXCLUDED.attribution_url,
		   updated_at = EXCLUDED.updated_at`,
		p.ID, p.Name, p.Slug, p.APIBaseURL, string(p.Status),
		p.AttributionText, p.AttributionURL, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert provider: %w", err)
	}
	return nil
}
