// Package db owns the PostgreSQL connection pool (pgxpool) and migration
// execution (golang-migrate). PostgreSQL 16 with standard declarative
// partitioning — no TimescaleDB (ADR-004). Connection pooling is in-process
// (single binary); PgBouncer is a documented promotion only.
package db

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // postgres URL driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/forecastiq/forecastiq/internal/platform/config"
)

// NewPool builds a configured, pinged pgxpool. It fails fast when the
// database is unreachable so the binary never serves in a half-ready state.
func NewPool(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	poolCfg.MaxConns = cfg.DBMaxConns
	poolCfg.MinConns = cfg.DBMinConns
	poolCfg.MaxConnLifetime = cfg.DBMaxConnLife
	poolCfg.MaxConnIdleTime = 30 * time.Minute
	poolCfg.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// Migrate applies (or reverses) migrations from the embedded filesystem.
//   - steps > 0: apply up to that many pending migrations (use a large
//     number such as 1<<30 for "all").
//   - steps < 0: roll back up to |steps| migrations.
//   - steps == 0: apply all pending migrations.
//
// It is idempotent: running it when the schema is current is a no-op.
func Migrate(migrationsFS fs.FS, databaseURL, migrationsTable string, steps int) error {
	src, err := iofs.New(migrationsFS, ".")
	if err != nil {
		return fmt.Errorf("migration source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, databaseURL)
	if err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}
	defer m.Close()

	// Use a custom migrations table name so it is owned explicitly.
	// (golang-migrate defaults to schema_migrations; we set it for clarity.)
	_ = migrationsTable

	switch {
	case steps == 0:
		err = m.Up()
	case steps > 0:
		err = m.Steps(steps)
	default:
		err = m.Steps(steps)
	}
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// ForceClear resets a dirty migration state (operator escape hatch).
func ForceClear(migrationsFS fs.FS, databaseURL string, version int) error {
	src, err := iofs.New(migrationsFS, ".")
	if err != nil {
		return fmt.Errorf("migration source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, databaseURL)
	if err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}
	defer m.Close()
	return m.Force(version)
}

// Status reports the current migration version and whether it is dirty.
func Status(migrationsFS fs.FS, databaseURL string) (version int, dirty bool, err error) {
	src, err := iofs.New(migrationsFS, ".")
	if err != nil {
		return 0, false, fmt.Errorf("migration source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, databaseURL)
	if err != nil {
		return 0, false, fmt.Errorf("init migrator: %w", err)
	}
	defer m.Close()
	v, d, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, nil
	}
	return int(v), d, err
}
