package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/forecastiq/forecastiq/adapters/forecastproviders/openmeteo"
	"github.com/forecastiq/forecastiq/adapters/payloadstore"
	"github.com/forecastiq/forecastiq/adapters/persistence/auditpg"
	"github.com/forecastiq/forecastiq/adapters/persistence/catalogpg"
	"github.com/forecastiq/forecastiq/adapters/persistence/collectionpg"
	"github.com/forecastiq/forecastiq/adapters/persistence/schedulerpg"
	"github.com/forecastiq/forecastiq/internal/api"
	"github.com/forecastiq/forecastiq/internal/api/handlers"
	"github.com/forecastiq/forecastiq/internal/audit"
	"github.com/forecastiq/forecastiq/internal/catalog"
	"github.com/forecastiq/forecastiq/internal/collection"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/config"
	"github.com/forecastiq/forecastiq/internal/platform/db"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
	"github.com/forecastiq/forecastiq/internal/platform/events"
	"github.com/forecastiq/forecastiq/internal/platform/health"
	"github.com/forecastiq/forecastiq/internal/platform/logging"
	"github.com/forecastiq/forecastiq/internal/platform/metrics"
	"github.com/forecastiq/forecastiq/internal/platform/ratelimit"
	"github.com/forecastiq/forecastiq/internal/scheduler"
)

// App is the wired application (composition root).
type App struct {
	cfg          config.Config
	logger       *slog.Logger
	pool         *pgxpool.Pool
	router       *gin.Engine
	metrics      *metrics.Metrics
	scheduler    *scheduler.Scheduler
	payloadStore *payloadstore.FilesystemStore
}

// buildApp loads configuration and wires every adapter to its port. This is
// the only place infrastructure imports converge (dependency rule).
func buildApp(ctx context.Context) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	logger := logging.New(cfg.LogLevel, cfg.LogFormat, nil)
	slog.SetDefault(logger)

	pool, err := db.NewPool(ctx, cfg)
	if err != nil {
		return nil, err
	}
	tx := dbtx.NewRunner(pool)
	clk := clock.Real{}

	// Persistence adapters.
	locationRepo := catalogpg.NewLocationRepository()
	providerRepo := catalogpg.NewProviderRepository()
	configRepo := catalogpg.NewConfigurationRepository()
	circuitRepo := catalogpg.NewCircuitRepository()
	collectionRepo := collectionpg.NewCollectionRepository()
	snapshotRepo := collectionpg.NewSnapshotRepository()
	slotRepo := schedulerpg.NewSlotRepository()
	runRepo := schedulerpg.NewRunRepository()
	auditStore := auditpg.NewStore()

	recorder := audit.NewRecorder(auditStore)

	// Catalog services.
	locations := catalog.NewLocationService(locationRepo, tx, pool, recorder, clk, logger)
	providers := catalog.NewProviderService(providerRepo, pool)
	configs := catalog.NewConfigurationService(configRepo, pool)
	circuits := catalog.NewCircuitService(circuitRepo, tx)

	// Payload store.
	payloadStore, err := payloadstore.NewFilesystemStore(cfg.PayloadStoreDir)
	if err != nil {
		pool.Close()
		return nil, err
	}

	// Event seam + subscribers.
	bus := events.NewSyncBus(logger)
	bus.Subscribe("forecast.collected", func(ctx context.Context, e events.Event) {
		if fc, ok := e.(events.ForecastCollected); ok {
			logger.InfoContext(ctx, "event.forecast.collected",
				slog.String("collection_id", fc.CollectionID.String()),
				slog.String("status", fc.Status),
				slog.Int("snapshot_count", fc.SnapshotCount))
		}
	})
	bus.Subscribe("provider.health_changed", func(ctx context.Context, e events.Event) {
		if hc, ok := e.(events.ProviderHealthChanged); ok {
			logger.WarnContext(ctx, "event.provider.health_changed",
				slog.String("provider_id", hc.ProviderID.String()),
				slog.String("old_state", hc.OldState),
				slog.String("new_state", hc.NewState),
				slog.Int("consecutive_failures", hc.ConsecutiveFailures))
		}
	})

	m := metrics.New()

	// Provider adapter (Open-Meteo; keyless at MVP).
	providerLimiter := ratelimit.NewLimiter(6, 6.0/60.0, clk) // 6 req/min effective
	omAdapter := openmeteo.New(openmeteo.Config{
		Client:           &http.Client{Timeout: cfg.ProviderTimeout},
		Limiter:          providerLimiter,
		Logger:           logger,
		MaxResponseBytes: cfg.ProviderMaxRespBytes,
		MaxRetries:       5,
		RetryBaseDelay:   time.Second,
	})
	// Provider registry: validates identity/versions and rejects duplicate
	// slugs at startup (WP-05). Add future providers here.
	registry := collection.NewRegistry()
	if err := registry.Register(omAdapter); err != nil {
		return nil, fmt.Errorf("register provider adapter: %w", err)
	}
	for _, d := range registry.Descriptors() {
		logger.Info("provider.registered",
			slog.String("provider", d.Slug),
			slog.String("schema_version", d.SchemaVersion),
			slog.String("adapter_version", d.AdapterVersion),
			slog.Duration("max_forecast_horizon", d.Capabilities.MaxForecastHorizon),
			slog.Bool("requires_credential", d.Capabilities.RequiresCredential),
			slog.Bool("supports_replay", d.Capabilities.SupportsReplay))
	}

	// Collection services.
	collector := collection.NewCollectService(
		registry.Adapters(), collectionRepo, snapshotRepo, payloadStore, circuits,
		bus, m, recorder, clk, logger, tx, pool, cfg.ResolveCredential)
	reader := collection.NewReaderService(collectionRepo, snapshotRepo, pool)

	// Health checks.
	checker := health.NewChecker()
	checker.Register("database", func(ctx context.Context) error {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		return pool.Ping(pingCtx)
	})
	checker.Register("payload_volume", func(ctx context.Context) error {
		return payloadStore.Writable()
	})

	// HTTP layer.
	ipLimiter := ratelimit.NewKeyedLimiter(float64(cfg.RateLimitIPPerMin), float64(cfg.RateLimitIPPerMin)/60.0, clk)
	h := &handlers.Handlers{
		Locations: locations, Providers: providers, Configs: configs,
		Collector: collector, Reader: reader, Health: checker, Logger: logger,
	}
	router := api.NewRouter(h, m, logger, api.RouterConfig{
		DevAdminToken:    cfg.DevAdminToken,
		CORSAllowOrigins: cfg.CORSAllowOrigins,
		RateLimiter:      ipLimiter,
	})

	// Scheduler / worker.
	dispatcher := scheduler.NewForecastDispatcher(configs, providers, locations, collector, logger)
	sched := scheduler.New(slotRepo, runRepo, dispatcher, configs, locations, tx, clk, logger, m, scheduler.Config{
		Interval:      cfg.SchedulerInterval,
		LeaseDuration: cfg.SlotLeaseDuration,
		MaxConcurrent: cfg.WorkerMaxConcurrent,
	})

	return &App{
		cfg: cfg, logger: logger, pool: pool, router: router,
		metrics: m, scheduler: sched, payloadStore: payloadStore,
	}, nil
}

// close releases resources.
func (a *App) close() {
	if a.pool != nil {
		a.pool.Close()
	}
}
