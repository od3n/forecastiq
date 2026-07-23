package identitypg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/forecastiq/forecastiq/internal/identity/domain"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// APIKeyRepository implements ports.APIKeyRepository.
type APIKeyRepository struct{}

// NewAPIKeyRepository returns an APIKeyRepository.
func NewAPIKeyRepository() *APIKeyRepository { return &APIKeyRepository{} }

// keyColumnsNoHash omits key_hash — the default for display/management paths so
// the secret hash never leaves the database except on the authentication path.
const keyColumnsNoHash = `id, user_id, workspace_id, name, key_prefix, scopes,
	rate_limit_per_min, expires_at, created_at, revoked_at, last_used_at`

func scanKeyNoHash(row pgx.Row) (*domain.APIKey, error) {
	var k domain.APIKey
	var scopes []byte
	err := row.Scan(&k.ID, &k.UserID, &k.WorkspaceID, &k.Name, &k.KeyPrefix, &scopes,
		&k.RateLimitPerMin, &k.ExpiresAt, &k.CreatedAt, &k.RevokedAt, &k.LastUsedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrKeyNotFound
		}
		return nil, fmt.Errorf("scan api key: %w", err)
	}
	if derr := decodeScopes(scopes, &k); derr != nil {
		return nil, derr
	}
	return &k, nil
}

func decodeScopes(scopes []byte, k *domain.APIKey) error {
	if len(scopes) > 0 {
		if err := json.Unmarshal(scopes, &k.Scopes); err != nil {
			return fmt.Errorf("decode scopes: %w", err)
		}
	}
	return nil
}

// Insert implements ports.APIKeyRepository.
func (r *APIKeyRepository) Insert(ctx context.Context, tx dbtx.DBTX, k *domain.APIKey) error {
	scopes, err := json.Marshal(k.Scopes)
	if err != nil {
		return fmt.Errorf("encode scopes: %w", err)
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO api_keys (id, user_id, workspace_id, name, key_hash, key_prefix,
		   scopes, rate_limit_per_min, expires_at, created_at, revoked_at, last_used_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		k.ID, k.UserID, k.WorkspaceID, k.Name, k.KeyHash, k.KeyPrefix,
		string(scopes), k.RateLimitPerMin, k.ExpiresAt, k.CreatedAt, k.RevokedAt, k.LastUsedAt)
	if err != nil {
		return fmt.Errorf("insert api key: %w", err)
	}
	return nil
}

// GetByPrefix implements ports.APIKeyRepository (authentication path — includes
// the hash).
func (r *APIKeyRepository) GetByPrefix(ctx context.Context, tx dbtx.DBTX, prefix string) (*domain.APIKey, error) {
	var k domain.APIKey
	var scopes []byte
	err := tx.QueryRow(ctx,
		`SELECT id, user_id, workspace_id, name, key_hash, key_prefix, scopes,
		   rate_limit_per_min, expires_at, created_at, revoked_at, last_used_at
		 FROM api_keys WHERE key_prefix = $1`, prefix).
		Scan(&k.ID, &k.UserID, &k.WorkspaceID, &k.Name, &k.KeyHash, &k.KeyPrefix, &scopes,
			&k.RateLimitPerMin, &k.ExpiresAt, &k.CreatedAt, &k.RevokedAt, &k.LastUsedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrKeyNotFound
		}
		return nil, fmt.Errorf("get api key by prefix: %w", err)
	}
	if derr := decodeScopes(scopes, &k); derr != nil {
		return nil, derr
	}
	return &k, nil
}

// ListByUser implements ports.APIKeyRepository (no hash).
func (r *APIKeyRepository) ListByUser(ctx context.Context, tx dbtx.DBTX, userID uuid.UUID) ([]*domain.APIKey, error) {
	rows, err := tx.Query(ctx,
		`SELECT `+keyColumnsNoHash+` FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()
	var out []*domain.APIKey
	for rows.Next() {
		k, serr := scanKeyNoHash(rows)
		if serr != nil {
			return nil, serr
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// GetByID implements ports.APIKeyRepository (no hash; ownership check).
func (r *APIKeyRepository) GetByID(ctx context.Context, tx dbtx.DBTX, id uuid.UUID) (*domain.APIKey, error) {
	return scanKeyNoHash(tx.QueryRow(ctx, `SELECT `+keyColumnsNoHash+` FROM api_keys WHERE id = $1`, id))
}

// Revoke implements ports.APIKeyRepository (idempotent; never reactivates).
func (r *APIKeyRepository) Revoke(ctx context.Context, tx dbtx.DBTX, id uuid.UUID, at time.Time) error {
	_, err := tx.Exec(ctx,
		`UPDATE api_keys SET revoked_at = $2 WHERE id = $1 AND revoked_at IS NULL`, id, at)
	if err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	return nil
}

// TouchLastUsed implements ports.APIKeyRepository.
func (r *APIKeyRepository) TouchLastUsed(ctx context.Context, tx dbtx.DBTX, id uuid.UUID, at time.Time) error {
	_, err := tx.Exec(ctx, `UPDATE api_keys SET last_used_at = $2 WHERE id = $1`, id, at)
	if err != nil {
		return fmt.Errorf("touch api key last used: %w", err)
	}
	return nil
}
