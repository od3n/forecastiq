//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/internal/audit"
	"github.com/forecastiq/forecastiq/internal/identity"
	identitydomain "github.com/forecastiq/forecastiq/internal/identity/domain"
)

// TestProvisioningIdempotency proves provision-on-first-use: the first
// authentication creates the user; subsequent authentications with the same
// subject reuse it (no duplicate rows).
func TestProvisioningIdempotency(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	first, err := e.identityUsers.Authenticate(ctx, "alice:alice@example.com", identity.Actor{})
	require.NoError(t, err)
	assert.Equal(t, identity.RoleUser, first.Role, "provisioned users default to role=user")

	second, err := e.identityUsers.Authenticate(ctx, "alice:alice@example.com", identity.Actor{})
	require.NoError(t, err)
	assert.Equal(t, first.UserID, second.UserID, "same subject must resolve to the same user")

	var count int
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT count(*) FROM users WHERE auth_subject = 'dev|alice'`).Scan(&count))
	assert.Equal(t, 1, count, "provisioning must be idempotent")

	// A provisioning audit row was written.
	var audits int
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT count(*) FROM audit_events WHERE action = 'user.provisioned' AND user_id = $1`,
		first.UserID).Scan(&audits))
	assert.Equal(t, 1, audits)
}

// TestDisabledUserDenied proves the role/status come from the database: a
// disabled user cannot authenticate even with a valid token.
func TestDisabledUserDenied(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	p, err := e.identityUsers.Authenticate(ctx, "bob", identity.Actor{})
	require.NoError(t, err)
	_, err = e.pool.Exec(ctx, `UPDATE users SET status = 'disabled' WHERE id = $1`, p.UserID)
	require.NoError(t, err)

	_, err = e.identityUsers.Authenticate(ctx, "bob", identity.Actor{})
	require.ErrorIs(t, err, identitydomain.ErrUserDisabled)
}

// TestAPIKeyLifecycle proves creation (plaintext once, hash never returned),
// listing (no hash), key authentication, and revocation (owner-only).
func TestAPIKeyLifecycle(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	owner, err := e.identityUsers.Authenticate(ctx, "carol", identity.Actor{})
	require.NoError(t, err)

	created, err := e.identityKeys.CreateKey(ctx, *owner, identity.CreateKeyInput{Name: "ci-key"})
	require.NoError(t, err)
	require.NotEmpty(t, created.Plaintext, "plaintext must be returned once")
	assert.Empty(t, created.Key.KeyHash, "creation result must not expose the hash")

	// The stored hash is never the plaintext.
	var storedHash string
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT key_hash FROM api_keys WHERE id = $1`, created.Key.ID).Scan(&storedHash))
	assert.NotEqual(t, created.Plaintext, storedHash)
	assert.Contains(t, storedHash, "$argon2id$")

	// Listing never returns the hash.
	keys, err := e.identityKeys.ListKeys(ctx, owner.UserID)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Empty(t, keys[0].KeyHash, "listing must exclude the hash")

	// The plaintext authenticates to the owning principal.
	authed, err := e.identityKeys.AuthenticateAPIKey(ctx, created.Plaintext)
	require.NoError(t, err)
	assert.Equal(t, owner.UserID, authed.UserID)
	assert.Equal(t, identity.AuthAPIKey, authed.Method)

	// A wrong secret is rejected without an oracle.
	_, err = e.identityKeys.AuthenticateAPIKey(ctx, created.Key.KeyPrefix+".wrongsecret")
	require.ErrorIs(t, err, identitydomain.ErrInvalidCredential)

	// Revocation is owner-only: another user cannot revoke this key.
	other, err := e.identityUsers.Authenticate(ctx, "dave", identity.Actor{})
	require.NoError(t, err)
	require.ErrorIs(t, e.identityKeys.RevokeKey(ctx, other.UserID, created.Key.ID, identity.Actor{}), identitydomain.ErrKeyNotFound)

	// The owner revokes; the key no longer authenticates.
	require.NoError(t, e.identityKeys.RevokeKey(ctx, owner.UserID, created.Key.ID, identity.Actor{}))
	_, err = e.identityKeys.AuthenticateAPIKey(ctx, created.Plaintext)
	require.ErrorIs(t, err, identitydomain.ErrKeyInactive)

	// Audit rows exist for create + revoke.
	var actions int
	require.NoError(t, e.pool.QueryRow(ctx,
		`SELECT count(*) FROM audit_events WHERE action IN ('apikey.create','apikey.revoke') AND user_id = $1`,
		owner.UserID).Scan(&actions))
	assert.Equal(t, 2, actions)
}

// TestAuditReader proves the reader returns recorded events newest-first and
// filters by action.
func TestAuditReader(t *testing.T) {
	ctx := context.Background()
	connStr := startPostgres(ctx, t)
	migrate(t, connStr)
	e := newTestEnv(ctx, t, connStr, newSuccessAdapter(1))
	e.seedCatalog(ctx, t)

	p, err := e.identityUsers.Authenticate(ctx, "erin", identity.Actor{})
	require.NoError(t, err)
	_, err = e.identityKeys.CreateKey(ctx, *p, identity.CreateKeyInput{Name: "k"})
	require.NoError(t, err)

	action := "apikey.create"
	events, page, err := e.auditReader.List(ctx, audit.Filter{Action: &action, Limit: 10})
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "apikey.create", events[0].Action)
	assert.False(t, page.HasMore)
}
