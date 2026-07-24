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
	DevAdminToken    string
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

		// Public catalog + data reads (cached: locations/providers class 300 s).
		v1.GET("/locations", catalogCache, h.ListLocations)
		v1.GET("/locations/:id", catalogCache, h.GetLocation)
		v1.GET("/providers", catalogCache, h.ListProviders)
		v1.GET("/providers/:id", catalogCache, h.GetProvider)
		v1.GET("/forecasts/latest", h.LatestForecast)

		// Public analysis reads (cached: rankings/accuracy class 60 s).
		v1.GET("/rankings", analysisCache, h.Rankings)
		v1.GET("/rankings/methodology", analysisCache, h.RankingsMethodology)
		v1.GET("/accuracy/summary", analysisCache, h.AccuracySummary)
		v1.GET("/accuracy", analysisCache, h.AccuracyTrends)
		// Forecast-vs-Actual (S-05). Cached at 60 s (today's date bound; a
		// per-date max-age=300 for past days is a documented follow-on).
		v1.GET("/forecast-comparison", analysisCache, h.ForecastComparison)

		// Admin mutations + lineage queries.
		admin := v1.Group("", RequireAdmin(cfg.DevAdminToken))
		{
			admin.POST("/locations", h.CreateLocation)
			admin.PUT("/locations/:id", h.UpdateLocation)
			admin.PATCH("/locations/:id/status", h.SetLocationStatus)
			admin.POST("/admin/collections/trigger", h.TriggerCollection)
			admin.POST("/admin/collections/:id/replay", h.ReplayCollection)
			admin.GET("/forecast-collections", h.ListCollections)
			admin.GET("/forecast-collections/:id", h.GetCollection)
			admin.GET("/admin/health", h.AdminHealth)
		}
	}
	return r
}

// serveOpenAPI returns the committed OpenAPI 3.1 document.
func serveOpenAPI(c *gin.Context) {
	c.Data(http.StatusOK, "application/json; charset=utf-8", openapi.Spec)
}
