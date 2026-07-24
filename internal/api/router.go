package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/forecastiq/forecastiq/api/openapi"
	"github.com/forecastiq/forecastiq/internal/api/handlers"
	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/metrics"
	"github.com/forecastiq/forecastiq/internal/platform/ratelimit"
)

// Cache TTLs by payload class (caching doc §2 / conventions §6).
const (
	// cacheTTLCatalog is the locations/providers class (rarely changes).
	cacheTTLCatalog = 300 * time.Second
	// cacheTTLAnalysis is the rankings/accuracy/methodology class (batch
	// writes refresh underlying rows within ~60 s of computation).
	cacheTTLAnalysis = 60 * time.Second
)

// RouterConfig configures the HTTP router.
type RouterConfig struct {
	Auth             Auth
	CORSAllowOrigins []string
	RateLimiter      *ratelimit.KeyedLimiter
	Clock            clock.Clock
}

// NewRouter builds the Gin engine with the middleware chain and first-slice
// routes. Operational probes (/healthz, /readyz) are unversioned; the API is
// URL-versioned under /api/v1 (API architecture §2). /metrics is served on a
// separate localhost-bound server (see cmd), not here.
func NewRouter(h *handlers.Handlers, m *metrics.Metrics, logger *slog.Logger, cfg RouterConfig) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(Recovery(logger), RequestID(), RequestLogger(logger), Metrics(m), CORS(cfg.CORSAllowOrigins))

	clk := cfg.Clock
	if clk == nil {
		clk = clock.Real{}
	}
	cache := NewResponseCache(LRUCapacity, clk)
	catalogCache := Cache(cache, m, clk, cacheTTLCatalog)
	analysisCache := Cache(cache, m, clk, cacheTTLAnalysis)

	// Operational probes (no auth, no rate limit).
	r.GET("/healthz", h.Healthz)
	r.GET("/readyz", h.Readyz)

	v1 := r.Group("/api/v1")
	v1.Use(RateLimit(cfg.RateLimiter))
	{
		v1.GET("/openapi.json", serveOpenAPI)

		// Signed auth-provider webhook receiver (public route, HMAC-gated;
		// audit-requirements §5). Mounted only when a secret is configured.
		if h.WebhookSecret != "" {
			v1.POST("/auth/webhook", h.AuthWebhook)
		}

		// Public catalog + data reads (cached: locations/providers class 300 s).
		v1.GET("/locations", catalogCache, h.ListLocations)
		v1.GET("/locations/:id", catalogCache, h.GetLocation)
		v1.GET("/providers", catalogCache, h.ListProviders)
		v1.GET("/providers/:id", catalogCache, h.GetProvider)

		// Public analysis reads (cached: rankings/accuracy class 60 s).
		v1.GET("/rankings", analysisCache, h.Rankings)
		v1.GET("/rankings/methodology", analysisCache, h.RankingsMethodology)
		v1.GET("/accuracy/summary", analysisCache, h.AccuracySummary)
		v1.GET("/accuracy", analysisCache, h.AccuracyTrends)
		// Forecast-vs-Actual (S-05). Cached at 60 s (today's date bound; a
		// per-date max-age=300 for past days is a documented follow-on).
		v1.GET("/forecast-comparison", analysisCache, h.ForecastComparison)

		// Raw forecast data is gated: authenticated user with the read:data
		// scope (AUTH-08 — the bulk provider surface is not public).
		v1.GET("/forecasts/latest", cfg.Auth.RequireAuth(), RequireScope("read:data"), h.LatestForecast)

		// Self-service (any authenticated user; personal data → no-store).
		self := v1.Group("", cfg.Auth.RequireAuth())
		{
			self.GET("/me", h.GetMe)
			self.PATCH("/me", h.UpdateMe)
			self.DELETE("/me", h.DeleteMe)
			self.POST("/me/export", h.RequestMyExport)
			self.GET("/exports/:id", h.DownloadExport)
			self.GET("/api-keys", h.ListAPIKeys)
			self.POST("/api-keys", h.CreateAPIKey)
			self.DELETE("/api-keys/:id", h.RevokeAPIKey)
		}

		// Admin mutations + lineage queries (authenticated + role=admin).
		admin := v1.Group("", cfg.Auth.RequireAuth(), RequireRole("admin"))
		{
			admin.POST("/locations", h.CreateLocation)
			admin.PUT("/locations/:id", h.UpdateLocation)
			admin.PATCH("/locations/:id/status", h.SetLocationStatus)
			admin.POST("/admin/collections/trigger", h.TriggerCollection)
			admin.POST("/admin/collections/:id/replay", h.ReplayCollection)
			admin.GET("/forecast-collections", h.ListCollections)
			admin.GET("/forecast-collections/:id", h.GetCollection)
			admin.GET("/admin/health", h.AdminHealth)
			admin.PATCH("/admin/providers/:id/status", h.SetProviderStatus)
			admin.PATCH("/admin/provider-configurations/:id", h.UpdateProviderConfiguration)
			admin.GET("/admin/audit-events", h.AuditEvents)
			admin.POST("/admin/recompute", h.AdminRecompute)
			admin.GET("/admin/users", h.ListUsers)
			admin.PATCH("/admin/users/:id/status", h.SetUserStatus)
			admin.DELETE("/admin/users/:id", h.DeleteUser)
			admin.POST("/admin/users/:id/export", h.RequestUserExport)
		}
	}
	return r
}

// serveOpenAPI returns the committed OpenAPI 3.1 document.
func serveOpenAPI(c *gin.Context) {
	c.Data(http.StatusOK, "application/json; charset=utf-8", openapi.Spec)
}
