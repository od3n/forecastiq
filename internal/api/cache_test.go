package api

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/metrics"
)

// newCacheEngine builds a minimal engine with the Cache middleware over a
// handler that counts invocations and echoes a query-dependent body.
func newCacheEngine(clk clock.Clock, ttl time.Duration, calls *atomic.Int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	m := metrics.New()
	r.GET("/api/v1/rankings", Cache(NewResponseCache(LRUCapacity, clk), m, clk, ttl), func(c *gin.Context) {
		calls.Add(1)
		c.JSON(http.StatusOK, gin.H{"location": c.Query("location_id")})
	})
	return r
}

func get(r *gin.Engine, target, ifNoneMatch string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestCache_MissThenHit(t *testing.T) {
	var calls atomic.Int64
	clk := clock.NewMutable(time.Unix(1_700_000_000, 0))
	r := newCacheEngine(clk, 60*time.Second, &calls)

	// First request: MISS — handler runs, 200, strong ETag + Cache-Control set.
	rec1 := get(r, "/api/v1/rankings?location_id=abc", "")
	require.Equal(t, http.StatusOK, rec1.Code)
	assert.EqualValues(t, 1, calls.Load())
	etag := rec1.Header().Get("ETag")
	require.NotEmpty(t, etag)
	assert.Equal(t, "public, max-age=60", rec1.Header().Get("Cache-Control"))

	// Second identical request: HIT — handler NOT re-run, identical body + ETag.
	rec2 := get(r, "/api/v1/rankings?location_id=abc", "")
	require.Equal(t, http.StatusOK, rec2.Code)
	assert.EqualValues(t, 1, calls.Load(), "handler must not run on a cache hit")
	assert.Equal(t, etag, rec2.Header().Get("ETag"))
	assert.Equal(t, rec1.Body.String(), rec2.Body.String())
}

func TestCache_ConditionalNotModified(t *testing.T) {
	var calls atomic.Int64
	clk := clock.NewMutable(time.Unix(1_700_000_000, 0))
	r := newCacheEngine(clk, 60*time.Second, &calls)

	rec1 := get(r, "/api/v1/rankings?location_id=abc", "")
	etag := rec1.Header().Get("ETag")

	// If-None-Match with the current ETag → 304, empty body, ETag echoed.
	rec2 := get(r, "/api/v1/rankings?location_id=abc", etag)
	assert.Equal(t, http.StatusNotModified, rec2.Code)
	assert.Empty(t, rec2.Body.String())
	assert.EqualValues(t, 1, calls.Load())
}

func TestCache_KeyIncludesSortedParams(t *testing.T) {
	var calls atomic.Int64
	clk := clock.NewMutable(time.Unix(1_700_000_000, 0))
	r := newCacheEngine(clk, 60*time.Second, &calls)

	// Different param values → different keys → two handler runs.
	get(r, "/api/v1/rankings?location_id=abc", "")
	get(r, "/api/v1/rankings?location_id=xyz", "")
	assert.EqualValues(t, 2, calls.Load())

	// Same params → hit (no third run).
	get(r, "/api/v1/rankings?location_id=abc", "")
	assert.EqualValues(t, 2, calls.Load())
}

func TestCache_TTLExpiry(t *testing.T) {
	var calls atomic.Int64
	clk := clock.NewMutable(time.Unix(1_700_000_000, 0))
	r := newCacheEngine(clk, 60*time.Second, &calls)

	get(r, "/api/v1/rankings?location_id=abc", "")
	assert.EqualValues(t, 1, calls.Load())

	clk.Advance(61 * time.Second) // entry expires lazily
	get(r, "/api/v1/rankings?location_id=abc", "")
	assert.EqualValues(t, 2, calls.Load(), "expired entry must re-run the handler")
}

func TestCache_NonOKNotStored(t *testing.T) {
	var calls atomic.Int64
	clk := clock.NewMutable(time.Unix(1_700_000_000, 0))
	store := NewResponseCache(LRUCapacity, clk)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/rankings", Cache(store, metrics.New(), clk, 60*time.Second), func(c *gin.Context) {
		calls.Add(1)
		c.JSON(http.StatusUnprocessableEntity, gin.H{"type": "validation"})
	})

	rec := get(r, "/api/v1/rankings?location_id=abc", "")
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Empty(t, rec.Header().Get("ETag"))
	assert.Zero(t, store.Len(), "errors must never be cached")

	// A second request re-runs the handler (nothing cached).
	get(r, "/api/v1/rankings?location_id=abc", "")
	assert.EqualValues(t, 2, calls.Load())
}

func TestResponseCache_LRUEviction(t *testing.T) {
	clk := clock.NewMutable(time.Unix(1_700_000_000, 0))
	c := NewResponseCache(2, clk)
	exp := clk.Now().Add(time.Minute)
	c.put(&cacheEntry{key: "a", expiresAt: exp})
	c.put(&cacheEntry{key: "b", expiresAt: exp})
	_, _ = c.get("a") // touch a → b is now least-recently-used
	c.put(&cacheEntry{key: "c", expiresAt: exp})

	assert.Equal(t, 2, c.Len())
	_, ok := c.get("b")
	assert.False(t, ok, "b should have been evicted as least-recently-used")
	_, ok = c.get("a")
	assert.True(t, ok)
	_, ok = c.get("c")
	assert.True(t, ok)
}
