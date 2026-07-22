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
