package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/internal/platform/config"
)

func setValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("FIQ_ENV", "development")
	t.Setenv("FIQ_MODE", "all")
	t.Setenv("FIQ_DATABASE_URL", "postgres://u:p@localhost:5432/db?sslmode=disable")
	t.Setenv("FIQ_DEV_ADMIN_TOKEN", "dev-admin-token")
}

func TestLoad_Valid(t *testing.T) {
	setValidEnv(t)
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, config.EnvDevelopment, cfg.Env)
	assert.Equal(t, config.ModeAll, cfg.Mode)
	assert.Equal(t, "dev-admin-token", cfg.DevAdminToken)
	assert.Equal(t, int32(20), cfg.DBMaxConns)
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	t.Setenv("FIQ_DATABASE_URL", "")
	t.Setenv("FIQ_ENV", "development")
	_, err := config.Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "FIQ_DATABASE_URL")
}

func TestLoad_InvalidEnv(t *testing.T) {
	setValidEnv(t)
	t.Setenv("FIQ_ENV", "staging")
	_, err := config.Load()
	assert.Error(t, err)
}

func TestLoad_InvalidMode(t *testing.T) {
	setValidEnv(t)
	t.Setenv("FIQ_MODE", "both")
	_, err := config.Load()
	assert.Error(t, err)
}

func TestLoad_ProductionRejectsDevToken(t *testing.T) {
	setValidEnv(t)
	t.Setenv("FIQ_ENV", "production")
	t.Setenv("FIQ_DEV_ADMIN_TOKEN", "should-not-be-set")
	_, err := config.Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "FIQ_DEV_ADMIN_TOKEN")
}

func TestLoad_InvalidDuration(t *testing.T) {
	setValidEnv(t)
	t.Setenv("FIQ_PROVIDER_TIMEOUT", "notaduration")
	_, err := config.Load()
	assert.Error(t, err)
}

func TestResolveCredential(t *testing.T) {
	setValidEnv(t)
	t.Setenv("FIQ_PROVIDER_OPENMETEO_API_KEY", "secret-value")
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "secret-value", cfg.ResolveCredential("FIQ_PROVIDER_OPENMETEO_API_KEY"))
	assert.Equal(t, "", cfg.ResolveCredential(""))
}
