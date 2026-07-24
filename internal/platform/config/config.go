// Package config implements 12-factor configuration loading with strict,
// fail-fast validation (docs/delivery/03-environments.md §5). All runtime
// configuration arrives via FIQ_-prefixed environment variables; a local
// .env / .env.local file is read as a convenience (real environment always
// wins). Secrets are never logged.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Environment is the runtime environment class.
type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvTest        Environment = "test"
	EnvProduction  Environment = "production"
)

// Mode is the single-binary run mode (ADR-013).
type Mode string

const (
	ModeAPI    Mode = "api"
	ModeWorker Mode = "worker"
	ModeAll    Mode = "all"
)

// Config is the fully-validated application configuration.
type Config struct {
	Env  Environment
	Mode Mode

	HTTPAddr          string
	MetricsAddr       string
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	ShutdownTimeout   time.Duration
	RequestBodyLimit  int64
	CORSAllowOrigins  []string
	RateLimitIPPerMin int

	LogLevel  string
	LogFormat string

	DatabaseURL     string
	DBMaxConns      int32
	DBMinConns      int32
	DBMaxConnLife   time.Duration
	AutoMigrate     bool
	AutoSeed        bool
	MigrationsTable string

	PayloadStoreDir string

	// BackupStatusFile is the JSON status file written by the backup/restore
	// scripts and read by /admin/health (WP-18). Empty disables the section.
	BackupStatusFile string

	ProviderTimeout        time.Duration
	ProviderMaxRespBytes   int64
	ProviderCredentialEnv  map[string]string // provider slug -> env var name
	OpenWeatherDailyBudget int               // WP-07: OpenWeather calls per UTC day (0 disables the guard)

	SchedulerInterval   time.Duration
	SlotLeaseDuration   time.Duration
	WorkerMaxConcurrent int
	WorkerJobTimeout    time.Duration
	SchedulerDrain      time.Duration
	SchedulerMissed     time.Duration

	DevAdminToken string

	// Auth (ADR-008 Supabase JWKS). AuthDevMode selects the local dev token
	// verifier (non-production only); otherwise the JWKS verifier is used.
	AuthJWKSURL  string
	AuthIssuer   string
	AuthAudience string
	AuthDevMode  bool

	// Bootstrap admin (ADR-017): the auth subject promoted to the admin role at
	// seed time so the operator surface is reachable ("first account seeded
	// admin"). In dev-mode the subject is the dev token prefixed with "dev|".
	AuthBootstrapAdminSubject string
	AuthBootstrapAdminEmail   string
}

// Load reads configuration from the environment (optionally seeding from a
// .env file) and validates it. It returns an error describing every invalid
// value so operators can fix all problems in one pass.
func Load() (Config, error) {
	loadDotenv(".env.local")
	loadDotenv(".env")

	var errs []string
	fail := func(format string, args ...any) { errs = append(errs, fmt.Sprintf(format, args...)) }

	cfg := Config{}

	// Environment & mode
	cfg.Env = Environment(getEnv("FIQ_ENV", "development"))
	switch cfg.Env {
	case EnvDevelopment, EnvTest, EnvProduction:
	default:
		fail("FIQ_ENV must be one of development|test|production, got %q", cfg.Env)
	}

	cfg.Mode = Mode(getEnv("FIQ_MODE", "all"))
	switch cfg.Mode {
	case ModeAPI, ModeWorker, ModeAll:
	default:
		fail("FIQ_MODE must be one of api|worker|all, got %q", cfg.Mode)
	}

	// HTTP
	cfg.HTTPAddr = getEnv("FIQ_HTTP_ADDR", "0.0.0.0:8080")
	cfg.MetricsAddr = getEnv("FIQ_METRICS_ADDR", "127.0.0.1:9090")
	cfg.ReadTimeout = getDuration("FIQ_HTTP_READ_TIMEOUT", 10*time.Second, &errs, "FIQ_HTTP_READ_TIMEOUT")
	cfg.WriteTimeout = getDuration("FIQ_HTTP_WRITE_TIMEOUT", 30*time.Second, &errs, "FIQ_HTTP_WRITE_TIMEOUT")
	cfg.ShutdownTimeout = getDuration("FIQ_HTTP_SHUTDOWN_TIMEOUT", 30*time.Second, &errs, "FIQ_HTTP_SHUTDOWN_TIMEOUT")
	cfg.RequestBodyLimit = 1 << 20 // 1 MB (security architecture §4)
	cors := getEnv("FIQ_CORS_ALLOW_ORIGINS", "http://localhost:3000")
	for _, o := range strings.Split(cors, ",") {
		if o = strings.TrimSpace(o); o != "" {
			cfg.CORSAllowOrigins = append(cfg.CORSAllowOrigins, o)
		}
	}
	cfg.RateLimitIPPerMin = getInt("FIQ_RATE_LIMIT_PER_IP_PER_MIN", 120, &errs, "FIQ_RATE_LIMIT_PER_IP_PER_MIN")
	if cfg.RateLimitIPPerMin <= 0 {
		fail("FIQ_RATE_LIMIT_PER_IP_PER_MIN must be > 0")
	}

	// Logging
	cfg.LogLevel = strings.ToLower(getEnv("FIQ_LOG_LEVEL", "info"))
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		fail("FIQ_LOG_LEVEL must be one of debug|info|warn|error, got %q", cfg.LogLevel)
	}
	cfg.LogFormat = strings.ToLower(getEnv("FIQ_LOG_FORMAT", "json"))
	switch cfg.LogFormat {
	case "json", "text":
	default:
		fail("FIQ_LOG_FORMAT must be one of json|text, got %q", cfg.LogFormat)
	}

	// Database
	cfg.DatabaseURL = getEnv("FIQ_DATABASE_URL", "")
	if cfg.DatabaseURL == "" {
		fail("FIQ_DATABASE_URL is required")
	} else if !strings.HasPrefix(cfg.DatabaseURL, "postgres://") && !strings.HasPrefix(cfg.DatabaseURL, "postgresql://") {
		fail("FIQ_DATABASE_URL must be a postgres:// URL")
	}
	cfg.DBMaxConns = int32(getInt("FIQ_DB_MAX_CONNS", 20, &errs, "FIQ_DB_MAX_CONNS"))
	cfg.DBMinConns = int32(getInt("FIQ_DB_MIN_CONNS", 2, &errs, "FIQ_DB_MIN_CONNS"))
	cfg.DBMaxConnLife = getDuration("FIQ_DB_MAX_CONN_LIFETIME", time.Hour, &errs, "FIQ_DB_MAX_CONN_LIFETIME")
	if cfg.DBMaxConns <= 0 || cfg.DBMinConns < 0 || cfg.DBMinConns > cfg.DBMaxConns {
		fail("FIQ_DB_MIN_CONNS/FIQ_DB_MAX_CONNS invalid (need 0 <= min <= max, max > 0)")
	}
	cfg.AutoMigrate = getBool("FIQ_AUTO_MIGRATE", false)
	cfg.AutoSeed = getBool("FIQ_AUTO_SEED", false)
	cfg.MigrationsTable = "schema_migrations"

	// Payload store
	cfg.PayloadStoreDir = getEnv("FIQ_PAYLOAD_STORE_DIR", "./data/payloads")
	cfg.BackupStatusFile = getEnv("FIQ_BACKUP_STATUS_FILE", "")

	// Provider
	cfg.ProviderTimeout = getDuration("FIQ_PROVIDER_TIMEOUT", 10*time.Second, &errs, "FIQ_PROVIDER_TIMEOUT")
	cfg.ProviderMaxRespBytes = int64(getInt("FIQ_PROVIDER_MAX_RESPONSE_BYTES", 10<<20, &errs, "FIQ_PROVIDER_MAX_RESPONSE_BYTES"))
	cfg.ProviderCredentialEnv = map[string]string{
		"open-meteo":  "FIQ_PROVIDER_OPENMETEO_API_KEY",
		"openweather": "FIQ_PROVIDER_OPENWEATHER_API_KEY",
	}
	cfg.OpenWeatherDailyBudget = getInt("FIQ_PROVIDER_OPENWEATHER_DAILY_BUDGET", 1000, &errs, "FIQ_PROVIDER_OPENWEATHER_DAILY_BUDGET")

	// Scheduler / worker
	cfg.SchedulerInterval = getDuration("FIQ_SCHEDULER_INTERVAL", 15*time.Second, &errs, "FIQ_SCHEDULER_INTERVAL")
	cfg.SlotLeaseDuration = getDuration("FIQ_SLOT_LEASE_DURATION", 5*time.Minute, &errs, "FIQ_SLOT_LEASE_DURATION")
	cfg.WorkerMaxConcurrent = getInt("FIQ_WORKER_MAX_CONCURRENT", 8, &errs, "FIQ_WORKER_MAX_CONCURRENT")
	if cfg.WorkerMaxConcurrent <= 0 {
		fail("FIQ_WORKER_MAX_CONCURRENT must be > 0")
	}
	cfg.WorkerJobTimeout = getDuration("FIQ_WORKER_JOB_TIMEOUT", 60*time.Second, &errs, "FIQ_WORKER_JOB_TIMEOUT")
	cfg.SchedulerDrain = getDuration("FIQ_SCHEDULER_DRAIN_TIMEOUT", 30*time.Second, &errs, "FIQ_SCHEDULER_DRAIN_TIMEOUT")
	cfg.SchedulerMissed = getDuration("FIQ_SCHEDULER_MISSED_THRESHOLD", 0, &errs, "FIQ_SCHEDULER_MISSED_THRESHOLD")

	// Dev-mode auth
	cfg.DevAdminToken = getEnv("FIQ_DEV_ADMIN_TOKEN", "")
	if cfg.Env == EnvProduction && cfg.DevAdminToken != "" {
		fail("FIQ_DEV_ADMIN_TOKEN must NOT be set in production (use Supabase JWKS auth)")
	}

	// Auth (ADR-008). JWKS URL is required in production; dev-mode auth is
	// forbidden there (the dev verifier is also compiled out of release builds).
	cfg.AuthJWKSURL = getEnv("FIQ_AUTH_JWKS_URL", "")
	cfg.AuthIssuer = getEnv("FIQ_AUTH_ISSUER", "")
	cfg.AuthAudience = getEnv("FIQ_AUTH_AUDIENCE", "")
	cfg.AuthDevMode = getBool("FIQ_AUTH_DEV_MODE", cfg.Env != EnvProduction)
	cfg.AuthBootstrapAdminSubject = getEnv("FIQ_BOOTSTRAP_ADMIN_SUBJECT", "")
	cfg.AuthBootstrapAdminEmail = getEnv("FIQ_BOOTSTRAP_ADMIN_EMAIL", "")
	if cfg.Env == EnvProduction {
		if cfg.AuthDevMode {
			fail("FIQ_AUTH_DEV_MODE must be false in production")
		}
		if cfg.AuthJWKSURL == "" {
			fail("FIQ_AUTH_JWKS_URL is required in production")
		}
	}

	if len(errs) > 0 {
		return Config{}, fmt.Errorf("invalid configuration:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return cfg, nil
}

// IsProduction reports whether the runtime environment is production.
func (c Config) IsProduction() bool { return c.Env == EnvProduction }

// ResolveCredential returns the secret value for a provider credential_ref.
// The database stores only the reference (env var name, BR-08); the secret
// itself lives exclusively in the environment. Empty when unset.
func (c Config) ResolveCredential(credentialRef string) string {
	if credentialRef == "" {
		return ""
	}
	return os.Getenv(credentialRef)
}

// ── helpers ───────────────────────────────────────────────────────────

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getDuration(key string, def time.Duration, errs *[]string, name string) time.Duration {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		*errs = append(*errs, fmt.Sprintf("%s is not a valid duration: %v", name, err))
		return def
	}
	return d
}

func getInt(key string, def int, errs *[]string, name string) int {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		*errs = append(*errs, fmt.Sprintf("%s is not a valid integer: %v", name, err))
		return def
	}
	return n
}

func getBool(key string, def bool) bool {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return def
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		return def
	}
	return b
}

// loadDotenv seeds the environment from a simple KEY=VALUE file. Values
// already present in the environment are never overridden (12-factor: the
// real environment wins). Missing files are ignored.
func loadDotenv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		v = strings.Trim(v, `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			_ = os.Setenv(k, v)
		}
	}
}
