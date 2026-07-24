//go:build integration

// Package integration holds database + API integration tests that run against
// a real PostgreSQL 16 (testcontainers). Run with: make test-integration.
package integration

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/forecastiq/forecastiq/adapters/auth/devauth"
	"github.com/forecastiq/forecastiq/adapters/auth/supabaseadmin"
	"github.com/forecastiq/forecastiq/adapters/payloadstore"
	"github.com/forecastiq/forecastiq/adapters/persistence/adminpg"
	"github.com/forecastiq/forecastiq/adapters/persistence/analysispg"
	"github.com/forecastiq/forecastiq/adapters/persistence/auditpg"
	"github.com/forecastiq/forecastiq/adapters/persistence/catalogpg"
	"github.com/forecastiq/forecastiq/adapters/persistence/collectionpg"
	"github.com/forecastiq/forecastiq/adapters/persistence/identitypg"
	"github.com/forecastiq/forecastiq/adapters/persistence/schedulerpg"
	"github.com/forecastiq/forecastiq/internal/admin"
	"github.com/forecastiq/forecastiq/internal/analysis"
	"github.com/forecastiq/forecastiq/internal/api"
	"github.com/forecastiq/forecastiq/internal/api/handlers"
	"github.com/forecastiq/forecastiq/internal/audit"
	"github.com/forecastiq/forecastiq/internal/catalog"
	catalogdomain "github.com/forecastiq/forecastiq/internal/catalog/domain"
	"github.com/forecastiq/forecastiq/internal/collection"
	collectiondomain "github.com/forecastiq/forecastiq/internal/collection/domain"
	"github.com/forecastiq/forecastiq/internal/collection/ports"
	"github.com/forecastiq/forecastiq/internal/identity"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/config"
	"github.com/forecastiq/forecastiq/internal/platform/db"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
	"github.com/forecastiq/forecastiq/internal/platform/events"
	"github.com/forecastiq/forecastiq/internal/platform/health"
	"github.com/forecastiq/forecastiq/internal/platform/metrics"
	"github.com/forecastiq/forecastiq/internal/platform/ratelimit"
	"github.com/forecastiq/forecastiq/internal/scheduler"
	"github.com/forecastiq/forecastiq/migrations"
)

// testWebhookSecret is the HMAC secret the test router mounts the auth-webhook
// receiver with (WP-19b).
const testWebhookSecret = "test-webhook-secret"

// testConfig builds a minimal config for the test pool.
func testConfig(connStr string) config.Config {
	return config.Config{
		DatabaseURL:   connStr,
		DBMaxConns:    5,
		DBMinConns:    1,
		DBMaxConnLife: time.Hour,
	}
}

// adminConnStr is the connection string for the single, package-wide PostgreSQL
// container's default database. It is used only to CREATE/DROP the per-test
// databases carved out of the shared container; tests never run against it
// directly. Populated once by TestMain.
var adminConnStr string

// dbCounter yields a process-unique suffix for per-test database names.
var dbCounter atomic.Int64

// TestMain owns the lifecycle of a single PostgreSQL 16 container shared by the
// whole integration package. Previously every test started and terminated its
// own container (~28 per run); under constrained CI the asynchronous teardown
// of one container overlapped the startup of the next, causing nondeterministic
// readiness timeouts and connection resets in unrelated tests (DRB-WP04-RR-002).
// A single container removes that churn while per-test databases (see
// startPostgres) preserve full isolation.
func TestMain(m *testing.M) {
	ctx := context.Background()
	container, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("forecastiq"),
		postgres.WithUsername("forecastiq"),
		postgres.WithPassword("forecastiq"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration: start shared postgres: %v\n", err)
		os.Exit(1)
	}

	adminConnStr, err = container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = container.Terminate(context.Background())
		fmt.Fprintf(os.Stderr, "integration: connection string: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	// Terminate once, synchronously, after all tests complete.
	_ = container.Terminate(context.Background())
	os.Exit(code)
}

// startPostgres provisions a fresh, uniquely-named database inside the shared
// container and returns its connection string. Each test therefore gets a
// pristine, fully-isolated database (migrated and seeded independently) without
// the cost and teardown races of a per-test container. The database is dropped
// on test cleanup with FORCE so any lingering connections cannot block removal.
func startPostgres(ctx context.Context, t *testing.T) string {
	t.Helper()
	dbName := fmt.Sprintf("it_%d_%d", os.Getpid(), dbCounter.Add(1))

	admin, err := pgx.Connect(ctx, adminConnStr)
	require.NoError(t, err)
	_, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize())
	require.NoError(t, err)
	require.NoError(t, admin.Close(ctx))

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		c, err := pgx.Connect(cleanupCtx, adminConnStr)
		if err != nil {
			return
		}
		defer func() { _ = c.Close(cleanupCtx) }()
		_, _ = c.Exec(cleanupCtx,
			"DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize()+" WITH (FORCE)")
	})

	return connStrForDB(t, adminConnStr, dbName)
}

// connStrForDB rewrites the database path of a base connection string.
func connStrForDB(t *testing.T, base, dbName string) string {
	t.Helper()
	u, err := url.Parse(base)
	require.NoError(t, err)
	u.Path = "/" + dbName
	return u.String()
}

// migrate applies all migrations to the database at connStr.
func migrate(t *testing.T, connStr string) {
	t.Helper()
	require.NoError(t, db.Migrate(migrations.FS, connStr, "schema_migrations", 0))
}

// newPool returns a pinged pool for connStr.
func newPool(ctx context.Context, t *testing.T, connStr string) *pgxpool.Pool {
	t.Helper()
	pool, err := db.NewPool(ctx, testConfig(connStr))
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// fakeAdapter is a deterministic forecast provider for integration tests. It
// builds `count` snapshots at issuedAt+(i+1)h so target_time > issued_at always
// holds regardless of the wall clock.
type fakeAdapter struct {
	count      int
	outcome    ports.Outcome
	rawPayload []byte
}

func (f *fakeAdapter) Slug() string           { return "open-meteo" }
func (f *fakeAdapter) SchemaVersion() string  { return "openmeteo-v1" }
func (f *fakeAdapter) AdapterVersion() string { return "1.0.0-test" }
func (f *fakeAdapter) Capabilities() ports.Capabilities {
	return ports.Capabilities{MaxForecastHorizon: 7 * 24 * time.Hour, HourlyResolution: true, SupportsReplay: true}
}
func (f *fakeAdapter) FetchForecast(_ context.Context, req ports.ForecastRequest) (*ports.ForecastResult, error) {
	raw := f.rawPayload
	if raw == nil {
		raw = []byte(`{"hourly":"test"}`)
	}
	res := &ports.ForecastResult{
		RawPayload: raw, Checksum: ports.Checksum(raw),
		HTTPStatusCode: 200, SchemaVersion: "openmeteo-v1", AdapterVersion: "1.0.0-test",
		IssuedAt: req.IssuedAt, RecordsReceived: f.count, Outcome: f.outcome,
	}
	if f.outcome == ports.OutcomeSuccess || f.outcome == ports.OutcomePartial {
		res.Snapshots = f.buildSnapshots(req)
	}
	return res, nil
}

// DecodeStored implements ports.ReplayDecoder: deterministically re-derive the
// same snapshots from stored bytes with no network metadata (replay path).
func (f *fakeAdapter) DecodeStored(_ context.Context, req ports.ForecastRequest, raw []byte) (*ports.ForecastResult, error) {
	res := &ports.ForecastResult{
		RawPayload: raw, Checksum: ports.Checksum(raw),
		SchemaVersion: "openmeteo-v1", AdapterVersion: "1.0.0-test",
		IssuedAt: req.IssuedAt, RecordsReceived: f.count, Outcome: ports.OutcomeSuccess,
	}
	res.Snapshots = f.buildSnapshots(req)
	return res, nil
}

// buildSnapshots produces f.count deterministic snapshots at issuedAt+(i+1)h.
func (f *fakeAdapter) buildSnapshots(req ports.ForecastRequest) []*collectiondomain.ForecastSnapshot {
	var out []*collectiondomain.ForecastSnapshot
	for i := 0; i < f.count; i++ {
		temp := 30.0 + float64(i)
		out = append(out, &collectiondomain.ForecastSnapshot{
			ID:                       uuid.Must(uuid.NewV7()),
			ProviderID:               req.ProviderID,
			LocationID:               req.LocationID,
			IssuedAt:                 req.IssuedAt,
			TargetTime:               req.IssuedAt.Add(time.Duration(i+1) * time.Hour),
			ForecastHorizonMinutes:   (i + 1) * 60,
			TemperatureC:             &temp,
			CanonicalConditionCode:   collectiondomain.ConditionCloudy,
			ConditionTaxonomyVersion: collectiondomain.ConditionTaxonomyVersion,
		})
	}
	return out
}

// newSuccessAdapter returns a fake adapter producing `count` valid snapshots.
func newSuccessAdapter(count int) *fakeAdapter {
	return &fakeAdapter{count: count, outcome: ports.OutcomeSuccess}
}

// testEnv is a fully-wired test environment backed by a real database.
type testEnv struct {
	pool      *pgxpool.Pool
	tx        *dbtx.Runner
	locations catalog.LocationManager
	providers catalog.ProviderCatalog
	configs   catalog.ConfigurationManager
	circuits  catalog.CircuitState
	collector collection.ForecastCollector
	replayer  collection.ForecastReplayer
	reader    collection.ForecastReader
	slots     *schedulerpgSlotRepo
	router    *gin.Engine
	adapter   *fakeAdapter
	store     *payloadstore.FilesystemStore

	identityUsers *identity.UserService
	identityKeys  *identity.APIKeyService
	auditReader   *audit.ReaderService
}

// schedulerpgSlotRepo aliases the concrete slot repo type for tests.
type schedulerpgSlotRepo = schedulerpg.SlotRepository

// newTestEnv wires services against a fresh migrated database with a fake adapter.
func newTestEnv(ctx context.Context, t *testing.T, connStr string, adapter *fakeAdapter) *testEnv {
	t.Helper()
	pool := newPool(ctx, t, connStr)
	tx := dbtx.NewRunner(pool)
	clk := clock.Real{}
	logger := slog.New(slog.NewTextHandler(io_discard{}, nil))

	locationRepo := catalogpg.NewLocationRepository()
	providerRepo := catalogpg.NewProviderRepository()
	configRepo := catalogpg.NewConfigurationRepository()
	circuitRepo := catalogpg.NewCircuitRepository()
	collectionRepo := collectionpg.NewCollectionRepository()
	snapshotRepo := collectionpg.NewSnapshotRepository()
	auditStore := auditpg.NewStore()
	recorder := audit.NewRecorder(auditStore)

	locations := catalog.NewLocationService(locationRepo, tx, pool, recorder, clk, logger)
	providers := catalog.NewProviderService(providerRepo, pool, tx, recorder, clk)
	configs := catalog.NewConfigurationService(configRepo, pool, tx, recorder, clk)
	circuits := catalog.NewCircuitService(circuitRepo, tx)

	store, err := payloadstore.NewFilesystemStore(t.TempDir())
	require.NoError(t, err)

	bus := events.NewSyncBus(logger)
	m := metrics.New()

	adapters := map[string]ports.ForecastProviderAdapter{"open-meteo": adapter}
	collector := collection.NewCollectService(adapters, providers, collectionRepo, snapshotRepo, store, circuits,
		bus, m, recorder, clk, logger, tx, pool, func(string) string { return "" })
	reader := collection.NewReaderService(collectionRepo, snapshotRepo, pool)

	checker := health.NewChecker()
	analysisRead := analysis.NewReadService(analysispg.NewReadRepository(), pool)
	adminHealth := admin.NewHealthService(adminpg.NewHealthRepository(), pool, nil, nil, clk)
	auditReader := audit.NewReaderService(auditStore, pool)
	analysisDispatcher := scheduler.NewAnalysisDispatcher(
		analysis.NewMatchService(analysispg.NewMatchRepository(), tx, pool, m, clk, logger),
		analysis.NewAggregateService(analysispg.NewMetricRepository(), tx, pool, m, clk, logger),
		analysis.NewRankService(analysispg.NewRankingRepository(), tx, pool, m, clk, logger), logger)
	recompute := admin.NewRecomputeService(analysisDispatcher, tx, recorder, clk)

	// Identity (WP-03/WP-19): dev verifier + services over the same DB, wired
	// into the auth middleware and the /me + /api-keys self-service handlers.
	userRepo := identitypg.NewUserRepository()
	apiKeyRepo := identitypg.NewAPIKeyRepository()
	identityUsers := identity.NewUserService(userRepo, devauth.New(clk), tx, pool, recorder, clk, logger, catalogdomain.SystemWorkspaceID)
	identityKeys := identity.NewAPIKeyService(apiKeyRepo, userRepo, tx, pool, recorder, clk, logger)
	adminUsers := identity.NewAdminUserService(userRepo, supabaseadmin.NewNoop(), tx, pool, recorder, clk, logger)

	h := &handlers.Handlers{
		Locations: locations, Providers: providers, Configs: configs,
		ProviderAdmin: providers, ConfigAdmin: configs,
		Collector: collector, Replayer: collector, Reader: reader, Analysis: analysisRead,
		AdminHealthReader: adminHealth, Audit: auditReader, Recompute: recompute,
		Users: identityUsers, Keys: identityKeys, UserAdmin: adminUsers,
		Webhook:       identity.NewWebhookService(userRepo, tx, pool, recorder, clk, logger),
		WebhookSecret: testWebhookSecret,
		Health:        checker, Logger: logger,
	}
	limiter := ratelimit.NewKeyedLimiter(1000, 1000, clk)
	router := api.NewRouter(h, m, logger, api.RouterConfig{
		Auth:        api.Auth{Users: identityUsers, Keys: identityKeys},
		RateLimiter: limiter,
	})

	// Seed the admin user backing the shared admin token (a dev-mode bearer
	// whose verified subject is "dev|test-admin-token") so existing admin tests
	// authenticate through the real middleware as role=admin.
	seedTestAdmin(ctx, t, pool)

	return &testEnv{
		pool: pool, tx: tx, locations: locations, providers: providers, configs: configs,
		circuits: circuits, collector: collector, replayer: collector, reader: reader,
		slots: schedulerpg.NewSlotRepository(), router: router, adapter: adapter, store: store,
		identityUsers: identityUsers, identityKeys: identityKeys, auditReader: auditReader,
	}
}

// seedTestAdmin provisions the admin user backing the shared admin token. The
// dev verifier maps the bearer "test-admin-token" to subject
// "dev|test-admin-token"; this row makes that principal role=admin. It runs in
// newTestEnv (before seedCatalog), so it upserts the system workspace first to
// satisfy the users_workspace_id FK. Idempotent.
func seedTestAdmin(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(ctx,
		`INSERT INTO workspaces (id, name, slug, status, created_at, updated_at)
		 VALUES ($1, 'System', 'system', 'active', now(), now())
		 ON CONFLICT (id) DO NOTHING`,
		catalogdomain.SystemWorkspaceID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO users (id, workspace_id, auth_subject, email, role, status, preferences, created_at, updated_at)
		 VALUES ($1, $2, 'dev|test-admin-token', 'admin@dev.local', 'admin', 'active', '{}', now(), now())
		 ON CONFLICT (auth_subject) DO UPDATE SET role = 'admin', status = 'active'`,
		uuid.New(), catalogdomain.SystemWorkspaceID)
	require.NoError(t, err)
}

// seedCatalog inserts the system workspace, open-meteo provider + config, and
// the JB location (mirrors cmd seed; idempotent).
func (e *testEnv) seedCatalog(ctx context.Context, t *testing.T) {
	t.Helper()
	now := time.Now().UTC()
	_, err := e.pool.Exec(ctx,
		`INSERT INTO workspaces (id, name, slug, status, created_at, updated_at)
		 VALUES ($1, 'System', 'system', 'active', $2, $2)
		 ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name`,
		catalogdomain.SystemWorkspaceID, now)
	require.NoError(t, err)

	providerRepo := catalogpg.NewProviderRepository()
	require.NoError(t, providerRepo.Upsert(ctx, e.pool, &catalogdomain.Provider{
		ID: catalogdomain.OpenMeteoProviderID, Name: "Open-Meteo", Slug: "open-meteo",
		APIBaseURL: "https://api.open-meteo.com", Status: catalogdomain.StatusActive,
		AttributionText: "Weather data by Open-Meteo.com", AttributionURL: "https://open-meteo.com/",
		CreatedAt: now, UpdatedAt: now,
	}))

	configRepo := catalogpg.NewConfigurationRepository()
	require.NoError(t, configRepo.Upsert(ctx, e.pool, &catalogdomain.ProviderConfiguration{
		ID: catalogdomain.OpenMeteoConfigID, WorkspaceID: catalogdomain.SystemWorkspaceID,
		ProviderID: catalogdomain.OpenMeteoProviderID, Status: catalogdomain.StatusActive,
		CollectionSchedule: catalogdomain.DefaultSchedule(), AdapterVersion: "1.0.0-test",
		ValidationState: "unvalidated", CreatedAt: now, UpdatedAt: now,
	}))

	locationRepo := catalogpg.NewLocationRepository()
	require.NoError(t, locationRepo.Insert(ctx, e.pool, &catalogdomain.Location{
		ID: catalogdomain.JohorBahruLocationID, WorkspaceID: catalogdomain.SystemWorkspaceID,
		Name: "Johor Bahru", Latitude: 1.4927, Longitude: 103.7414,
		CountryCode: "MY", Timezone: "Asia/Kuala_Lumpur", Status: catalogdomain.StatusActive,
		CreatedAt: now, UpdatedAt: now,
	}))
}

// io_discard is a minimal slog sink that discards output.
type io_discard struct{}

func (io_discard) Write(p []byte) (int, error) { return len(p), nil }

// mustUUIDv7 returns a fresh UUIDv7 (test helper).
func mustUUIDv7() uuid.UUID { return uuid.Must(uuid.NewV7()) }

// monthStartsOf returns the distinct first-of-month instants covering the
// snapshots' target times (for partition creation).
func monthStartsOf(snaps []*collectiondomain.ForecastSnapshot) []time.Time {
	seen := map[time.Time]struct{}{}
	var out []time.Time
	for _, s := range snaps {
		tt := s.TargetTime.UTC()
		ms := time.Date(tt.Year(), tt.Month(), 1, 0, 0, 0, 0, time.UTC)
		if _, ok := seen[ms]; !ok {
			seen[ms] = struct{}{}
			out = append(out, ms)
		}
	}
	return out
}
