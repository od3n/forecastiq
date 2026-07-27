package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/forecastiq/forecastiq/adapters/persistence/catalogpg"
	catalogdomain "github.com/forecastiq/forecastiq/internal/catalog/domain"
	"github.com/forecastiq/forecastiq/internal/platform/config"
	"github.com/forecastiq/forecastiq/internal/platform/db"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
	"github.com/forecastiq/forecastiq/internal/platform/ids"
)

// cmdSeed seeds reference data (idempotent).
func cmdSeed(_ []string) error {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	pool, err := db.NewPool(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := seed(ctx, pool); err != nil {
		return err
	}
	if err := seedBootstrapAdmin(ctx, pool, cfg.AuthBootstrapAdminSubject, cfg.AuthBootstrapAdminEmail); err != nil {
		return err
	}
	fmt.Println("seed completed")
	return nil
}

// seed inserts the system workspace, seeded providers, the Open-Meteo
// configuration, and the Johor Bahru demo location. Every statement is
// idempotent (ON CONFLICT / existence check) so it is safe on every boot.
func seed(ctx context.Context, pool dbtx.DBTX) error {
	now := time.Now().UTC()

	// System workspace.
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspaces (id, name, slug, status, created_at, updated_at)
		 VALUES ($1, 'System', 'system', 'active', $2, $2)
		 ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, updated_at = EXCLUDED.updated_at`,
		catalogdomain.SystemWorkspaceID, now); err != nil {
		return fmt.Errorf("seed workspace: %w", err)
	}

	// Providers (open-meteo operational; openweather catalog metadata only).
	providerRepo := catalogpg.NewProviderRepository()
	for _, p := range []*catalogdomain.Provider{
		{
			ID: catalogdomain.OpenMeteoProviderID, Name: "Open-Meteo", Slug: "open-meteo",
			APIBaseURL: "https://api.open-meteo.com", Status: catalogdomain.StatusActive,
			AttributionText: "Weather data by Open-Meteo.com", AttributionURL: "https://open-meteo.com/",
			CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: catalogdomain.OpenWeatherProviderID, Name: "OpenWeather", Slug: "openweather",
			APIBaseURL: "https://api.openweathermap.org", Status: catalogdomain.StatusActive,
			AttributionText: "Weather data by OpenWeather", AttributionURL: "https://openweathermap.org/",
			CreatedAt: now, UpdatedAt: now,
		},
	} {
		if err := providerRepo.Upsert(ctx, pool, p); err != nil {
			return fmt.Errorf("seed provider %s: %w", p.Slug, err)
		}
	}

	// Open-Meteo operational configuration (keyless at MVP).
	configRepo := catalogpg.NewConfigurationRepository()
	cfg := &catalogdomain.ProviderConfiguration{
		ID: catalogdomain.OpenMeteoConfigID, WorkspaceID: catalogdomain.SystemWorkspaceID,
		ProviderID: catalogdomain.OpenMeteoProviderID, Status: catalogdomain.StatusActive,
		CredentialRef: "", CollectionSchedule: catalogdomain.DefaultSchedule(),
		AdapterVersion: "1.0.0", ValidationState: "unvalidated", CreatedAt: now, UpdatedAt: now,
	}
	if err := configRepo.Upsert(ctx, pool, cfg); err != nil {
		return fmt.Errorf("seed configuration: %w", err)
	}

	// OpenWeather operational configuration (WP-07). Seeded DISABLED: the
	// adapter and wiring are ready, but automatic collection is gated on the
	// OpenWeather ToS review (D-05, a public-launch gate) and a resolvable
	// FIQ_PROVIDER_OPENWEATHER_API_KEY. An operator activates it once both are
	// satisfied; while disabled the scheduler does not generate its slots.
	// Insert-only (existence check): re-seeding must never clobber an
	// operator's activation of the configuration.
	if _, lookupErr := configRepo.GetByID(ctx, pool, catalogdomain.OpenWeatherConfigID); errors.Is(lookupErr, catalogdomain.ErrNotFound) {
		owCfg := &catalogdomain.ProviderConfiguration{
			ID: catalogdomain.OpenWeatherConfigID, WorkspaceID: catalogdomain.SystemWorkspaceID,
			ProviderID: catalogdomain.OpenWeatherProviderID, Status: catalogdomain.StatusDisabled,
			CredentialRef: "FIQ_PROVIDER_OPENWEATHER_API_KEY", CollectionSchedule: catalogdomain.Schedule{Interval: "hourly", MinuteOffset: 2},
			AdapterVersion: "1.0.0", ValidationState: "unvalidated", CreatedAt: now, UpdatedAt: now,
		}
		if err := configRepo.Upsert(ctx, pool, owCfg); err != nil {
			return fmt.Errorf("seed openweather configuration: %w", err)
		}
	} else if lookupErr != nil {
		return fmt.Errorf("seed openweather configuration lookup: %w", lookupErr)
	}

	// Johor Bahru demo location (idempotent via existence check).
	locationRepo := catalogpg.NewLocationRepository()
	if _, lookupErr := locationRepo.GetByID(ctx, pool, catalogdomain.JohorBahruLocationID); errors.Is(lookupErr, catalogdomain.ErrNotFound) {
		jb := &catalogdomain.Location{
			ID: catalogdomain.JohorBahruLocationID, WorkspaceID: catalogdomain.SystemWorkspaceID,
			Name: "Johor Bahru", Latitude: 1.4927, Longitude: 103.7414,
			CountryCode: "MY", Timezone: "Asia/Kuala_Lumpur", Status: catalogdomain.StatusActive,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := locationRepo.Insert(ctx, pool, jb); err != nil {
			return fmt.Errorf("seed location: %w", err)
		}
	} else if lookupErr != nil {
		return fmt.Errorf("seed location lookup: %w", lookupErr)
	}

	return nil
}

// seedBootstrapAdmin promotes (or provisions) the configured auth subject to the
// admin role so the operator surface is reachable ("first account seeded admin";
// ADR-017). A no-op when unset. Idempotent: re-running re-asserts role=admin +
// active. The subject must match what the active verifier asserts (in dev-mode
// the dev token prefixed with "dev|"; in production the Supabase user id).
func seedBootstrapAdmin(ctx context.Context, pool dbtx.DBTX, subject, email string) error {
	if subject == "" {
		return nil
	}
	if email == "" {
		email = "admin@forecastiq.local"
	}
	now := time.Now().UTC()
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, workspace_id, auth_subject, email, role, status,
		   preferences, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'admin', 'active', '{}', $5, $5)
		 ON CONFLICT (auth_subject)
		   DO UPDATE SET role = 'admin', status = 'active', updated_at = EXCLUDED.updated_at`,
		ids.New(), catalogdomain.SystemWorkspaceID, subject, email, now); err != nil {
		return fmt.Errorf("seed bootstrap admin: %w", err)
	}
	return nil
}
