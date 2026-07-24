// Package identitypg implements the identity module's repository ports on
// PostgreSQL (pgx). jsonb columns (preferences, scopes) are marshaled as JSON
// text; a unique violation on auth_subject is translated to
// domain.ErrDuplicateSubject so the service can recover from a provisioning
// race. Wired only in cmd/ (composition root); parameterized queries throughout.
package identitypg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/forecastiq/forecastiq/internal/identity/domain"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// UserRepository implements ports.UserRepository.
type UserRepository struct{}

// NewUserRepository returns a UserRepository.
func NewUserRepository() *UserRepository { return &UserRepository{} }

const userColumns = `id, workspace_id, auth_subject, email, role, status,
	default_location_id, preferences, created_at, updated_at, last_login_at`

func scanUser(row pgx.Row) (*domain.User, error) {
	var u domain.User
	var role, status string
	var prefs []byte
	err := row.Scan(&u.ID, &u.WorkspaceID, &u.AuthSubject, &u.Email, &role, &status,
		&u.DefaultLocationID, &prefs, &u.CreatedAt, &u.UpdatedAt, &u.LastLoginAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("scan user: %w", err)
	}
	u.Role = domain.Role(role)
	u.Status = status
	if len(prefs) > 0 {
		if uerr := json.Unmarshal(prefs, &u.Preferences); uerr != nil {
			return nil, fmt.Errorf("decode preferences: %w", uerr)
		}
	}
	if u.Preferences == nil {
		u.Preferences = map[string]any{}
	}
	return &u, nil
}

// GetByID implements ports.UserRepository.
func (r *UserRepository) GetByID(ctx context.Context, tx dbtx.DBTX, id uuid.UUID) (*domain.User, error) {
	return scanUser(tx.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
}

// GetByAuthSubject implements ports.UserRepository.
func (r *UserRepository) GetByAuthSubject(ctx context.Context, tx dbtx.DBTX, subject string) (*domain.User, error) {
	return scanUser(tx.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE auth_subject = $1`, subject))
}

// Insert implements ports.UserRepository. A unique violation (auth_subject or
// email) becomes domain.ErrDuplicateSubject.
func (r *UserRepository) Insert(ctx context.Context, tx dbtx.DBTX, u *domain.User) error {
	prefs, err := json.Marshal(orEmptyMap(u.Preferences))
	if err != nil {
		return fmt.Errorf("encode preferences: %w", err)
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO users (id, workspace_id, auth_subject, email, role, status,
		   default_location_id, preferences, created_at, updated_at, last_login_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		u.ID, u.WorkspaceID, u.AuthSubject, u.Email, string(u.Role), u.Status,
		u.DefaultLocationID, string(prefs), u.CreatedAt, u.UpdatedAt, u.LastLoginAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateSubject
		}
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

// UpdateProfile implements ports.UserRepository (mutable fields only).
func (r *UserRepository) UpdateProfile(ctx context.Context, tx dbtx.DBTX, u *domain.User) error {
	prefs, err := json.Marshal(orEmptyMap(u.Preferences))
	if err != nil {
		return fmt.Errorf("encode preferences: %w", err)
	}
	_, err = tx.Exec(ctx,
		`UPDATE users SET default_location_id = $2, preferences = $3 WHERE id = $1`,
		u.ID, u.DefaultLocationID, string(prefs))
	if err != nil {
		return fmt.Errorf("update user profile: %w", err)
	}
	return nil
}

// TouchLastLogin implements ports.UserRepository.
func (r *UserRepository) TouchLastLogin(ctx context.Context, tx dbtx.DBTX, id uuid.UUID, at time.Time) error {
	_, err := tx.Exec(ctx, `UPDATE users SET last_login_at = $2 WHERE id = $1`, id, at)
	if err != nil {
		return fmt.Errorf("touch last login: %w", err)
	}
	return nil
}

// List implements ports.UserRepository (admin S-14; keyset by id, newest first).
func (r *UserRepository) List(ctx context.Context, tx dbtx.DBTX, limit int, cursor uuid.UUID) ([]*domain.User, error) {
	var rows pgx.Rows
	var err error
	if cursor == uuid.Nil {
		rows, err = tx.Query(ctx, `SELECT `+userColumns+` FROM users ORDER BY id DESC LIMIT $1`, limit)
	} else {
		rows, err = tx.Query(ctx, `SELECT `+userColumns+` FROM users WHERE id < $1 ORDER BY id DESC LIMIT $2`, cursor, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	var out []*domain.User
	for rows.Next() {
		u, serr := scanUser(rows)
		if serr != nil {
			return nil, serr
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// UpdateStatus implements ports.UserRepository (admin lifecycle).
func (r *UserRepository) UpdateStatus(ctx context.Context, tx dbtx.DBTX, id uuid.UUID, status string) error {
	ct, err := tx.Exec(ctx, `UPDATE users SET status = $2 WHERE id = $1`, id, status)
	if err != nil {
		return fmt.Errorf("update user status: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

// Delete implements ports.UserRepository (AUTH-09). api_keys cascade; audit
// rows are anonymized by the FK (ON DELETE SET NULL) — the caller must set the
// app.allow_immutable_write GUC so the immutability trigger permits it.
func (r *UserRepository) Delete(ctx context.Context, tx dbtx.DBTX, id uuid.UUID) error {
	ct, err := tx.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func orEmptyMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}
