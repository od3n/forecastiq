// Catalog seeding, dataset reset, and partition management (WP-26b).
//
// Catalog writes are INSERT-only (ON CONFLICT DO NOTHING) so re-running the
// seeder never clobbers operator state — the same convention as `forecastiq
// seed` (see fix/seed-insert-only-configs). Data tables are append-only with
// immutability triggers, so re-seeding requires an explicit --reset TRUNCATE
// (TRUNCATE does not fire the row-level immutability triggers).
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	catalogdomain "github.com/forecastiq/forecastiq/internal/catalog/domain"
	"github.com/forecastiq/forecastiq/test/perf/perfids"
)

// providerRef is one seeded provider + its operational configuration.
type providerRef struct {
	ProviderID uuid.UUID
	ConfigID   uuid.UUID
	Slug       string
}

// perfLocationID returns the deterministic id of perf location i (shared with
// the PT harnesses via perfids).
func perfLocationID(i int) uuid.UUID { return perfids.LocationID(i) }

// perfProviderID / perfConfigID cover providers beyond the two canonical ones.
func perfProviderID(i int) uuid.UUID { return perfids.ProviderID(i) }

func perfConfigID(i int) uuid.UUID { return perfids.ConfigID(i) }

// buildProviders maps provider index → catalog ids. The first two reuse the
// canonical Open-Meteo / OpenWeather seed ids so perf data flows through the
// same rows the application serves; extras get deterministic perf ids.
func buildProviders(n int) []providerRef {
	refs := make([]providerRef, 0, n)
	for i := 0; i < n; i++ {
		switch i {
		case 0:
			refs = append(refs, providerRef{catalogdomain.OpenMeteoProviderID, catalogdomain.OpenMeteoConfigID, "open-meteo"})
		case 1:
			refs = append(refs, providerRef{catalogdomain.OpenWeatherProviderID, catalogdomain.OpenWeatherConfigID, "openweather"})
		default:
			refs = append(refs, providerRef{perfProviderID(i), perfConfigID(i), fmt.Sprintf("perf-provider-%02d", i)})
		}
	}
	return refs
}

// ensureCatalog inserts (insert-only) the workspace, providers, configurations,
// perf locations, and the perf admin user backing ADMIN_TOKEN=perf-admin-token
// in dev-verifier environments.
func ensureCatalog(ctx context.Context, conn *pgx.Conn, provs []providerRef, nLocations int) error {
	now := time.Now().UTC()

	if _, err := conn.Exec(ctx,
		`INSERT INTO workspaces (id, name, slug, status, created_at, updated_at)
		 VALUES ($1, 'System', 'system', 'active', $2, $2)
		 ON CONFLICT (id) DO NOTHING`,
		catalogdomain.SystemWorkspaceID, now); err != nil {
		return fmt.Errorf("workspace: %w", err)
	}

	for i, p := range provs {
		if _, err := conn.Exec(ctx,
			`INSERT INTO providers (id, name, slug, api_base_url, status, attribution_text, attribution_url, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, 'active', $5, $6, $7, $7)
			 ON CONFLICT (id) DO NOTHING`,
			p.ProviderID, fmt.Sprintf("Perf Provider %02d", i), p.Slug,
			"https://perf.invalid", "Synthetic perf data", "https://perf.invalid", now); err != nil {
			return fmt.Errorf("provider %s: %w", p.Slug, err)
		}
		if _, err := conn.Exec(ctx,
			`INSERT INTO provider_configurations
			   (id, workspace_id, provider_id, status, collection_schedule, adapter_version, validation_state, created_at, updated_at)
			 VALUES ($1, $2, $3, 'active', '{"interval":"hourly","minute_offset":0}', '1.0.0-perf', 'unvalidated', $4, $4)
			 ON CONFLICT (id) DO NOTHING`,
			p.ConfigID, catalogdomain.SystemWorkspaceID, p.ProviderID, now); err != nil {
			return fmt.Errorf("configuration %s: %w", p.Slug, err)
		}
	}

	for i := 0; i < nLocations; i++ {
		// Tropical grid: ~1.3–6.3°N stepping east across Malaysia; spacing far
		// above the 0.05° dedup radius.
		lat := 1.3 + 0.5*float64(i%10)
		lon := 101.5 + 0.6*float64(i/10) + 0.11*float64(i%10)
		if _, err := conn.Exec(ctx,
			`INSERT INTO locations (id, workspace_id, name, latitude, longitude, country_code, timezone, status, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, 'MY', 'Asia/Kuala_Lumpur', 'active', $6, $6)
			 ON CONFLICT (id) DO NOTHING`,
			perfLocationID(i), catalogdomain.SystemWorkspaceID,
			fmt.Sprintf("Perf City %02d", i), lat, lon, now); err != nil {
			return fmt.Errorf("location %d: %w", i, err)
		}
	}

	// Perf admin (dev verifier maps bearer "perf-admin-token" to this subject).
	// Insert-only; harmless outside dev because the dev verifier is
	// build-tag-excluded from release binaries.
	if _, err := conn.Exec(ctx,
		`INSERT INTO users (id, workspace_id, auth_subject, email, role, status, preferences, created_at, updated_at)
		 VALUES ($1, $2, 'dev|perf-admin-token', 'perf-admin@dev.local', 'admin', 'active', '{}', $3, $3)
		 ON CONFLICT (auth_subject) DO NOTHING`,
		perfids.AdminUserID, catalogdomain.SystemWorkspaceID, now); err != nil {
		return fmt.Errorf("perf admin: %w", err)
	}
	return nil
}

// ensurePartitions creates the monthly partitions of forecast_snapshots and
// observations covering [from, to] via the migration's idempotent helper.
func ensurePartitions(ctx context.Context, conn *pgx.Conn, from, to time.Time) error {
	ms := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(to.Year(), to.Month(), 1, 0, 0, 0, 0, time.UTC)
	for !ms.After(end) {
		for _, tbl := range []string{"forecast_snapshots", "observations"} {
			if _, err := conn.Exec(ctx, `SELECT create_monthly_partition($1, $2::date)`, tbl, ms); err != nil {
				return fmt.Errorf("partition %s %s: %w", tbl, ms.Format("2006-01"), err)
			}
		}
		ms = ms.AddDate(0, 1, 0)
	}
	return nil
}

// dataTables are the perf-seeded data tables in TRUNCATE order.
var dataTables = []string{
	"provider_rankings", "accuracy_metrics", "matched_evaluations",
	"observations", "forecast_snapshots", "forecast_collections",
}

// resetData truncates the data tables (catalog and identity rows are kept).
// TRUNCATE is the only way to clear them: the row-level immutability triggers
// forbid DELETE by design.
func resetData(ctx context.Context, conn *pgx.Conn) error {
	for _, tbl := range dataTables {
		if _, err := conn.Exec(ctx, "TRUNCATE "+tbl+" CASCADE"); err != nil {
			return fmt.Errorf("truncate %s: %w", tbl, err)
		}
	}
	return nil
}

// dataPresent reports whether any perf-relevant data table already has rows
// (guard against duplicate-key COPY failures on accidental re-runs).
func dataPresent(ctx context.Context, conn *pgx.Conn) (bool, error) {
	for _, tbl := range dataTables {
		var one int
		err := conn.QueryRow(ctx, "SELECT 1 FROM "+tbl+" LIMIT 1").Scan(&one)
		if err == nil {
			return true, nil
		}
		if err != pgx.ErrNoRows {
			return false, fmt.Errorf("probe %s: %w", tbl, err)
		}
	}
	return false, nil
}
