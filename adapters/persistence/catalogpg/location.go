// Package catalogpg implements the catalog module's repository ports on
// PostgreSQL (pgx). It is wired to the catalog services only in cmd/ (the
// composition root); modules depend on the ports, never this package
// (binding rule, depguard-enforced). All queries are parameterized (no
// string SQL — security architecture §3).
package catalogpg

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/forecastiq/forecastiq/internal/catalog/domain"
	"github.com/forecastiq/forecastiq/internal/catalog/ports"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// LocationRepository implements ports.LocationRepository.
type LocationRepository struct{}

// NewLocationRepository returns a LocationRepository.
func NewLocationRepository() *LocationRepository { return &LocationRepository{} }

const locationColumns = `id, workspace_id, name, latitude, longitude, country_code, timezone, status, created_at, updated_at`

func scanLocation(row pgx.Row) (*domain.Location, error) {
	var l domain.Location
	var status string
	err := row.Scan(&l.ID, &l.WorkspaceID, &l.Name, &l.Latitude, &l.Longitude,
		&l.CountryCode, &l.Timezone, &status, &l.CreatedAt, &l.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("scan location: %w", err)
	}
	l.Status = domain.Status(status)
	return &l, nil
}

// Insert implements ports.LocationRepository.
func (r *LocationRepository) Insert(ctx context.Context, tx dbtx.DBTX, l *domain.Location) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO locations (`+locationColumns+`)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		l.ID, l.WorkspaceID, l.Name, l.Latitude, l.Longitude,
		l.CountryCode, l.Timezone, string(l.Status), l.CreatedAt, l.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert location: %w", err)
	}
	return nil
}

// GetByID implements ports.LocationRepository.
func (r *LocationRepository) GetByID(ctx context.Context, tx dbtx.DBTX, id uuid.UUID) (*domain.Location, error) {
	return scanLocation(tx.QueryRow(ctx, `SELECT `+locationColumns+` FROM locations WHERE id = $1`, id))
}

// List implements ports.LocationRepository (keyset pagination by id).
func (r *LocationRepository) List(ctx context.Context, tx dbtx.DBTX, f ports.LocationFilter) ([]*domain.Location, error) {
	query := `SELECT ` + locationColumns + ` FROM locations WHERE id > $1`
	args := []any{f.Cursor}
	if f.Active != nil {
		if *f.Active {
			query += ` AND status = 'active'`
		} else {
			query += ` AND status <> 'active'`
		}
	}
	query += ` ORDER BY id ASC LIMIT $2`
	rows, err := tx.Query(ctx, query, args[0], f.Limit+1)
	if err != nil {
		return nil, fmt.Errorf("list locations: %w", err)
	}
	defer rows.Close()
	return collectLocations(rows)
}

// ListActive implements ports.LocationRepository.
func (r *LocationRepository) ListActive(ctx context.Context, tx dbtx.DBTX) ([]*domain.Location, error) {
	rows, err := tx.Query(ctx, `SELECT `+locationColumns+` FROM locations WHERE status = 'active' ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list active locations: %w", err)
	}
	defer rows.Close()
	return collectLocations(rows)
}

// Update implements ports.LocationRepository (mutable fields only).
func (r *LocationRepository) Update(ctx context.Context, tx dbtx.DBTX, l *domain.Location) error {
	_, err := tx.Exec(ctx,
		`UPDATE locations SET name = $2, timezone = $3 WHERE id = $1`,
		l.ID, l.Name, l.Timezone)
	if err != nil {
		return fmt.Errorf("update location: %w", err)
	}
	return nil
}

// UpdateStatus implements ports.LocationRepository.
func (r *LocationRepository) UpdateStatus(ctx context.Context, tx dbtx.DBTX, id uuid.UUID, status domain.Status) error {
	_, err := tx.Exec(ctx, `UPDATE locations SET status = $2 WHERE id = $1`, id, string(status))
	if err != nil {
		return fmt.Errorf("update location status: %w", err)
	}
	return nil
}

func collectLocations(rows pgx.Rows) ([]*domain.Location, error) {
	var out []*domain.Location
	for rows.Next() {
		var l domain.Location
		var status string
		if err := rows.Scan(&l.ID, &l.WorkspaceID, &l.Name, &l.Latitude, &l.Longitude,
			&l.CountryCode, &l.Timezone, &status, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan location row: %w", err)
		}
		l.Status = domain.Status(status)
		out = append(out, &l)
	}
	return out, rows.Err()
}
