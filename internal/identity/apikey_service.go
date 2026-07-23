package identity

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/forecastiq/forecastiq/internal/audit"
	"github.com/forecastiq/forecastiq/internal/identity/domain"
	"github.com/forecastiq/forecastiq/internal/identity/ports"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
	"github.com/forecastiq/forecastiq/internal/platform/ids"
)

const (
	maxKeyNameLen     = 100
	defaultKeyRateMin = 60
)

// APIKeyService manages personal API keys: creation (argon2id, plaintext shown
// once), listing (never returns the hash), revocation (owner-only), and
// key-based authentication.
type APIKeyService struct {
	keys   ports.APIKeyRepository
	users  ports.UserRepository
	tx     *dbtx.Runner
	pool   dbtx.DBTX
	audit  audit.Recorder
	clock  clock.Clock
	logger *slog.Logger
}

// NewAPIKeyService wires an APIKeyService.
func NewAPIKeyService(keys ports.APIKeyRepository, users ports.UserRepository, tx *dbtx.Runner,
	pool dbtx.DBTX, rec audit.Recorder, clk clock.Clock, logger *slog.Logger) *APIKeyService {
	return &APIKeyService{keys: keys, users: users, tx: tx, pool: pool, audit: rec, clock: clk, logger: logger}
}

// nameError is a field-validation error surfaced as 422 by the API layer.
type nameError struct{ msg string }

func (e *nameError) Error() string   { return e.msg }
func (e *nameError) Field() string   { return "name" }
func (e *nameError) Message() string { return e.msg }

// CreateKey mints a new API key for the principal, returning the plaintext
// exactly once. Only the argon2id hash is persisted.
func (s *APIKeyService) CreateKey(ctx context.Context, p Principal, in CreateKeyInput) (*CreatedKey, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" || len(name) > maxKeyNameLen {
		return nil, &nameError{msg: fmt.Sprintf("name is required and must be at most %d characters", maxKeyNameLen)}
	}
	prefix, plaintext, err := generateKey()
	if err != nil {
		return nil, err
	}
	hash, err := hashKey(plaintext)
	if err != nil {
		return nil, err
	}

	scopes := in.Scopes
	if len(scopes) == 0 {
		scopes = domain.DefaultScopes()
	}
	rate := in.RateLimitPerMin
	if rate <= 0 {
		rate = defaultKeyRateMin
	}
	now := s.clock.Now()
	key := &domain.APIKey{
		ID: ids.New(), UserID: p.UserID, WorkspaceID: p.WorkspaceID,
		Name: name, KeyHash: hash, KeyPrefix: prefix, Scopes: scopes,
		RateLimitPerMin: rate, ExpiresAt: in.ExpiresAt, CreatedAt: now,
	}
	if terr := s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		if ierr := s.keys.Insert(ctx, tx, key); ierr != nil {
			return ierr
		}
		return s.audit.Record(ctx, tx, audit.Event{
			UserID: &p.UserID, Action: "apikey.create", ResourceType: "api_key",
			ResourceID: &key.ID, IPAddress: in.Actor.IPAddress,
			Details: map[string]any{"name": name, "key_prefix": prefix, "scopes": scopes}, At: now,
		})
	}); terr != nil {
		return nil, fmt.Errorf("create api key: %w", terr)
	}
	s.logger.InfoContext(ctx, "apikey.created",
		slog.String("user_id", p.UserID.String()), slog.String("key_prefix", prefix))

	// Return a hash-free copy so the plaintext is the only secret surfaced.
	out := *key
	out.KeyHash = ""
	return &CreatedKey{Key: &out, Plaintext: plaintext}, nil
}

// ListKeys returns the principal's keys without hashes.
func (s *APIKeyService) ListKeys(ctx context.Context, userID uuid.UUID) ([]*domain.APIKey, error) {
	return s.keys.ListByUser(ctx, s.pool, userID)
}

// RevokeKey revokes a key the principal owns. Unknown or non-owned keys return
// ErrKeyNotFound (no existence disclosure to non-owners).
func (s *APIKeyService) RevokeKey(ctx context.Context, userID, keyID uuid.UUID, actor Actor) error {
	now := s.clock.Now()
	return s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		key, err := s.keys.GetByID(ctx, tx, keyID)
		if err != nil {
			return err // ErrKeyNotFound
		}
		if key.UserID != userID {
			return domain.ErrKeyNotFound
		}
		if rerr := s.keys.Revoke(ctx, tx, keyID, now); rerr != nil {
			return rerr
		}
		return s.audit.Record(ctx, tx, audit.Event{
			UserID: &userID, Action: "apikey.revoke", ResourceType: "api_key",
			ResourceID: &keyID, IPAddress: actor.IPAddress,
			Details: map[string]any{"key_prefix": key.KeyPrefix}, At: now,
		})
	})
}

// AuthenticateAPIKey resolves a presented plaintext key to a principal:
// prefix lookup → constant-time hash verify → usable + active checks → role
// from the database. Any failure returns ErrInvalidCredential without
// distinguishing the cause (no oracle).
func (s *APIKeyService) AuthenticateAPIKey(ctx context.Context, rawKey string) (*Principal, error) {
	prefix, ok := splitKey(rawKey)
	if !ok {
		return nil, domain.ErrInvalidCredential
	}
	key, err := s.keys.GetByPrefix(ctx, s.pool, prefix)
	if err != nil {
		if errors.Is(err, domain.ErrKeyNotFound) {
			return nil, domain.ErrInvalidCredential
		}
		return nil, err
	}
	match, err := verifyKey(rawKey, key.KeyHash)
	if err != nil || !match {
		return nil, domain.ErrInvalidCredential
	}
	now := s.clock.Now()
	if !key.Usable(now) {
		return nil, domain.ErrKeyInactive
	}
	user, err := s.users.GetByID(ctx, s.pool, key.UserID)
	if err != nil {
		return nil, err
	}
	if !user.Active() {
		return nil, domain.ErrUserDisabled
	}
	// Best-effort last-used timestamp; a failure must not deny a valid key.
	_ = s.tx.Run(ctx, func(ctx context.Context, tx dbtx.DBTX) error {
		return s.keys.TouchLastUsed(ctx, tx, key.ID, now)
	})
	return &Principal{
		UserID: user.ID, WorkspaceID: user.WorkspaceID, Email: user.Email,
		Role: user.Role, Method: AuthAPIKey, Scopes: key.Scopes,
	}, nil
}
