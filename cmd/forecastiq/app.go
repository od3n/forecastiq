package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/forecastiq/forecastiq/adapters/auth/devauth"
	"github.com/forecastiq/forecastiq/adapters/auth/jwks"
	"github.com/forecastiq/forecastiq/adapters/auth/supabaseadmin"
	"github.com/forecastiq/forecastiq/adapters/backupstatus"
	"github.com/forecastiq/forecastiq/adapters/forecastproviders/openmeteo"
	"github.com/forecastiq/forecastiq/adapters/forecastproviders/openweather"
	obsopenmeteo "github.com/forecastiq/forecastiq/adapters/observationsources/openmeteo"
	"github.com/forecastiq/forecastiq/adapters/payloadstore"
	"github.com/forecastiq/forecastiq/adapters/persistence/adminpg"
	"github.com/forecastiq/forecastiq/adapters/persistence/analysispg"
	"github.com/forecastiq/forecastiq/adapters/persistence/auditpg"
	"github.com/forecastiq/forecastiq/adapters/persistence/catalogpg"
	"github.com/forecastiq/forecastiq/adapters/persistence/collectionpg"
	"github.com/forecastiq/forecastiq/adapters/persistence/identitypg"
	"github.com/forecastiq/forecastiq/adapters/persistence/observationpg"
	"github.com/forecastiq/forecastiq/adapters/persistence/schedulerpg"
	"github.com/forecastiq/forecastiq/internal/admin"
	"github.com/forecastiq/forecastiq/internal/analysis"
	"github.com/forecastiq/forecastiq/internal/api"
	"github.com/forecastiq/forecastiq/internal/api/handlers"
	"github.com/forecastiq/forecastiq/internal/audit"
	"github.com/forecastiq/forecastiq/internal/catalog"
	catalogdomain "github.com/forecastiq/forecastiq/internal/catalog/domain"
	"github.com/forecastiq/forecastiq/internal/collection"
	"github.com/forecastiq/forecastiq/internal/identity"
	"github.com/forecastiq/forecastiq/internal/identity/ports"
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

	// Identity (WP-03): use cases + verifier are wired here; HTTP routes that
	// consume them land in WP-15/WP-19.
	identityUsers *identity.UserService
	identityKeys  *identity.APIKeyService
	auditReader   *audit.ReaderService
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
	providers := catalog.NewProviderService(providerRepo, pool, tx, recorder, clk)
	configs := catalog.NewConfigurationService(configRepo, pool, tx, recorder, clk)
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
	bus.Subscribe("observation.collected", func(ctx context.Context, e events.Event) {
		if oc, ok := e.(events.ObservationCollected); ok {
			logger.InfoContext(ctx, "event.observation.collected",
				slog.String("location_id", oc.LocationID.String()),
				slog.String("source", oc.Source),
				slog.Int("count", oc.Count))
		}
	})
	bus.Subscribe("observation.corrected", func(ctx context.Context, e events.Event) {
		if oc, ok := e.(events.ObservationCorrected); ok {
			logger.InfoContext(ctx, "event.observation.corrected",
				slog.String("location_id", oc.LocationID.String()),
				slog.String("superseded_observation_id", oc.SupersededObservationID.String()),
				slog.String("new_observation_id", oc.NewObservationID.String()))
		}
	})

	m := metrics.New()
	m.RegisterPoolCollector(pool)

	// Payload volume gauges (architecture §3.6 Runtime): report statfs bytes at
	// each Prometheus scrape via the payloadStore.Usage() call. Errors degrade
	// to reporting zero rather than failing the scrape.
	m.Registry.MustRegister(
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "payload_volume_used_bytes",
			Help: "Payload storage volume used bytes.",
		}, func() float64 {
			u, err := payloadStore.Usage()
			if err != nil {
				return 0
			}
			return float64(u.UsedBytes)
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "payload_volume_total_bytes",
			Help: "Payload storage volume total capacity bytes.",
		}, func() float64 {
			u, err := payloadStore.Usage()
			if err != nil {
				return 0
			}
			return float64(u.TotalBytes)
		}),
	)

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
	// OpenWeather One Call 3.0 (WP-07; ToS-gated for public launch — the seeded
	// configuration ships disabled). The daily rate-budget guard caps calls per
	// UTC day and pauses on a 429 (429 → pause); a modest per-minute bucket
	// smooths outbound bursts.
	owLimiter := ratelimit.NewLimiter(6, 6.0/60.0, clk) // 6 req/min effective
	owAdapter := openweather.New(openweather.Config{
		Client:           &http.Client{Timeout: cfg.ProviderTimeout},
		Limiter:          owLimiter,
		Logger:           logger,
		Clock:            clk,
		MaxResponseBytes: cfg.ProviderMaxRespBytes,
		MaxRetries:       5,
		RetryBaseDelay:   time.Second,
		DailyBudget:      cfg.OpenWeatherDailyBudget,
	})
	if err := registry.Register(owAdapter); err != nil {
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
		registry.Adapters(), providers, collectionRepo, snapshotRepo, payloadStore, circuits,
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
	analysisRead := analysis.NewReadService(analysispg.NewReadRepository(), pool)
	adminHealth := admin.NewHealthService(adminpg.NewHealthRepository(), pool,
		payloadVolumeStater{store: payloadStore}, backupstatus.New(cfg.BackupStatusFile), clk)

	// Identity (WP-03/WP-19). The verifier is the Supabase JWKS verifier in
	// production; a local dev verifier otherwise (compiled out of release
	// builds). Provisioned users belong to the system workspace (ADR-009). The
	// services back the auth middleware (RequireAuth/Role/Scope) and the
	// /me + /api-keys self-service handlers, so they are built before the router.
	var verifier ports.TokenVerifier
	if cfg.AuthDevMode {
		verifier = devauth.New(clk)
		logger.Warn("auth.dev_mode_enabled")
	} else {
		verifier = jwks.New(jwks.Config{
			JWKSURL: cfg.AuthJWKSURL, Issuer: cfg.AuthIssuer, Audience: cfg.AuthAudience,
		})
	}
	userRepo := identitypg.NewUserRepository()
	apiKeyRepo := identitypg.NewAPIKeyRepository()
	identityUsers := identity.NewUserService(userRepo, verifier, tx, pool, recorder, clk, logger, catalogdomain.SystemWorkspaceID)
	identityKeys := identity.NewAPIKeyService(apiKeyRepo, userRepo, tx, pool, recorder, clk, logger)
	// Supabase Admin API propagation (ADR-008 §6; WP-19b): a no-op in dev/test or
	// when unconfigured; the real client otherwise. Account disable/delete stay
	// authoritative locally regardless.
	var supabaseAdmin ports.SupabaseAdmin
	if cfg.AuthDevMode || cfg.SupabaseServiceRoleKey == "" {
		supabaseAdmin = supabaseadmin.NewNoop()
	} else {
		supabaseAdmin = supabaseadmin.New(supabaseadmin.Config{
			ProjectURL: cfg.SupabaseProjectURL, ServiceRoleKey: cfg.SupabaseServiceRoleKey,
		})
	}
	adminUsers := identity.NewAdminUserService(userRepo, supabaseAdmin, tx, pool, recorder, clk, logger)
	auditReader := audit.NewReaderService(auditStore, pool)
	exports := identity.NewExportService(identitypg.NewExportJobRepository(), userRepo, apiKeyRepo,
		auditReader, payloadStore, tx, pool, recorder, clk, logger, catalogdomain.SystemWorkspaceID)
	logger.Info("identity.ready", slog.Bool("dev_mode", cfg.AuthDevMode))

	h := &handlers.Handlers{
		Locations: locations, Providers: providers, Configs: configs,
		ProviderAdmin: providers, ConfigAdmin: configs,
		Collector: collector, Replayer: collector, Reader: reader, Analysis: analysisRead,
		AdminHealthReader: adminHealth, Audit: auditReader,
		Users: identityUsers, Keys: identityKeys, UserAdmin: adminUsers,
		Webhook:       identity.NewWebhookService(userRepo, tx, pool, recorder, clk, logger),
		WebhookSecret: cfg.AuthWebhookSecret,
		Exports:       exports,
		Health:        checker, Logger: logger,
	}
	router := api.NewRouter(h, m, logger, api.RouterConfig{
		Auth:             api.Auth{Users: identityUsers, Keys: identityKeys},
		CORSAllowOrigins: cfg.CORSAllowOrigins,
		RateLimiter:      ipLimiter,
		Clock:            clk,
	})

	// Scheduler / worker.
	dispatcher := scheduler.NewForecastDispatcher(configs, providers, locations, collector, logger)
	// Observation collection (WP-10): Open-Meteo Historical source adapter →
	// ObserveService → dispatched at :05 per active location. Observation slots
	// hang off the seeded Open-Meteo config (job_type discriminates).
	obsLimiter := ratelimit.NewLimiter(6, 6.0/60.0, clk)
	obsAdapter := obsopenmeteo.New(obsopenmeteo.Config{
		Client:           &http.Client{Timeout: cfg.ProviderTimeout},
		Limiter:          obsLimiter,
		Logger:           logger,
		MaxResponseBytes: cfg.ProviderMaxRespBytes,
		MaxRetries:       5,
		RetryBaseDelay:   time.Second,
	})
	observationRepo := observationpg.NewRepository()
	observer := collection.NewObserveService(obsAdapter, observationRepo, bus, m, clk, logger, tx, pool)
	obsDispatcher := scheduler.NewObservationDispatcher(observer, locations, logger)
	// Analysis matching engine (WP-11): analysis_batch at :10/:40 pairs unmatched
	// snapshots with observations and rematches superseded pairs.
	matcher := analysis.NewMatchService(analysispg.NewMatchRepository(), tx, pool, m, clk, logger)
	// Analysis aggregation (WP-13): the same batch aggregates matched pairs into
	// AccuracyMetric rows (per cell-period, with CIs, coverage/reliability).
	aggregator := analysis.NewAggregateService(analysispg.NewMetricRepository(), tx, pool, m, clk, logger)
	// Analysis ranking (WP-14): the batch then ranks providers per cell into
	// ProviderRanking rows (cohort normalization, weights, penalty, statuses).
	ranker := analysis.NewRankService(analysispg.NewRankingRepository(), tx, pool, m, clk, logger)
	analysisDispatcher := scheduler.NewAnalysisDispatcher(matcher, aggregator, ranker, m, logger)
	jobRouter := scheduler.NewRouter(map[string]scheduler.Dispatcher{
		scheduler.JobForecastCollection:    dispatcher,
		scheduler.JobObservationCollection: obsDispatcher,
		scheduler.JobAnalysisBatch:         analysisDispatcher,
	})
	sched := scheduler.New(slotRepo, runRepo, jobRouter, configs, locations, tx, clk, logger, m, scheduler.Config{
		Interval:                cfg.SchedulerInterval,
		LeaseDuration:           cfg.SlotLeaseDuration,
		MaxConcurrent:           cfg.WorkerMaxConcurrent,
		JobTimeout:              cfg.WorkerJobTimeout,
		DrainTimeout:            cfg.SchedulerDrain,
		MissedThreshold:         cfg.SchedulerMissed,
		ObservationConfigID:     catalogdomain.OpenMeteoConfigID,
		ObservationMinuteOffset: 5,
		AnalysisConfigID:        catalogdomain.OpenMeteoConfigID,
	})

	// Identity (WP-03/WP-19) is constructed with the HTTP layer above (the auth
	// middleware + self-service handlers depend on it).

	// Admin recompute wired now that the analysis dispatcher exists (handlers
	// read this field at request time).
	h.Recompute = admin.NewRecomputeService(analysisDispatcher, tx, recorder, clk)

	return &App{
		cfg: cfg, logger: logger, pool: pool, router: router,
		metrics: m, scheduler: sched, payloadStore: payloadStore,
		identityUsers: identityUsers, identityKeys: identityKeys, auditReader: auditReader,
	}, nil
}

// close releases resources.
func (a *App) close() {
	if a.pool != nil {
		a.pool.Close()
	}
}

// payloadVolumeStater adapts the filesystem payload store to admin.VolumeStater
// (S-10 system section), mapping the store's usage struct to the admin type
// without coupling the admin module to the storage adapter.
type payloadVolumeStater struct{ store *payloadstore.FilesystemStore }

func (v payloadVolumeStater) Usage() (admin.VolumeUsage, error) {
	u, err := v.store.Usage()
	if err != nil {
		return admin.VolumeUsage{}, err
	}
	return admin.VolumeUsage{UsedBytes: u.UsedBytes, TotalBytes: u.TotalBytes, UsedPct: u.UsedPct}, nil
}
