package api

import (
	"bytes"
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/forecastiq/forecastiq/internal/platform/clock"
	"github.com/forecastiq/forecastiq/internal/platform/metrics"
)

// LRUCapacity is the fixed entry budget of the read-path cache (caching doc §1:
// 256 entries ≈ 20 MB worst case at the 80 KB max response).
const LRUCapacity = 256

// cacheEntry is one stored response (body + strong ETag + content type),
// expiring lazily at expiresAt.
type cacheEntry struct {
	key         string
	body        []byte
	etag        string
	contentType string
	expiresAt   time.Time
}

// ResponseCache is an in-process, TTL-bounded LRU of serialized GET responses
// keyed by route + canonical params + auth class (caching doc §1). It has no
// external dependency (no Redis at MVP; constraints §3). Thread-safe.
type ResponseCache struct {
	mu    sync.Mutex
	cap   int
	ll    *list.List               // front = most-recently-used
	items map[string]*list.Element // key → element(*cacheEntry)
	clock clock.Clock
}

// NewResponseCache builds a ResponseCache with the given capacity.
func NewResponseCache(capacity int, clk clock.Clock) *ResponseCache {
	if capacity <= 0 {
		capacity = LRUCapacity
	}
	return &ResponseCache{
		cap:   capacity,
		ll:    list.New(),
		items: make(map[string]*list.Element, capacity),
		clock: clk,
	}
}

// get returns a live (non-expired) entry, promoting it to most-recently-used.
// Expired entries are evicted lazily on lookup.
func (c *ResponseCache) get(key string) (*cacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return nil, false
	}
	e := el.Value.(*cacheEntry)
	if c.clock.Now().After(e.expiresAt) {
		c.ll.Remove(el)
		delete(c.items, key)
		return nil, false
	}
	c.ll.MoveToFront(el)
	return e, true
}

// put stores (or refreshes) an entry, evicting the least-recently-used entry
// when at capacity.
func (c *ResponseCache) put(e *cacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[e.key]; ok {
		el.Value = e
		c.ll.MoveToFront(el)
		return
	}
	c.items[e.key] = c.ll.PushFront(e)
	for c.ll.Len() > c.cap {
		oldest := c.ll.Back()
		if oldest == nil {
			break
		}
		c.ll.Remove(oldest)
		delete(c.items, oldest.Value.(*cacheEntry).key)
	}
}

// Len reports the current number of cached entries (test/introspection).
func (c *ResponseCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ll.Len()
}

// cacheKey derives the LRU key: route template + canonical sorted query params
// + auth class. Public endpoints share one entry (never per-user; caching §1).
func cacheKey(c *gin.Context, authClass string) string {
	q := c.Request.URL.Query()
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteString(c.Request.Method)
	sb.WriteByte('\n')
	sb.WriteString(c.FullPath())
	sb.WriteByte('\n')
	for _, k := range keys {
		vs := append([]string(nil), q[k]...)
		sort.Strings(vs)
		for _, v := range vs {
			sb.WriteString(k)
			sb.WriteByte('=')
			sb.WriteString(v)
			sb.WriteByte('&')
		}
	}
	sb.WriteByte('\n')
	sb.WriteString(authClass)
	sum := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(sum[:])
}

// bufferWriter buffers the handler's response so the middleware can compute a
// content-based ETag and decide 200-vs-304 before anything is flushed. Nothing
// reaches the underlying connection until flush().
type bufferWriter struct {
	gin.ResponseWriter
	status int
	buf    bytes.Buffer
}

func (w *bufferWriter) Write(b []byte) (int, error)       { return w.buf.Write(b) }
func (w *bufferWriter) WriteString(s string) (int, error) { return w.buf.WriteString(s) }
func (w *bufferWriter) WriteHeader(code int)              { w.status = code }
func (w *bufferWriter) WriteHeaderNow()                   {} // defer flush to the middleware
func (w *bufferWriter) Written() bool                     { return false }
func (w *bufferWriter) Size() int                         { return w.buf.Len() }
func (w *bufferWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

// flush writes the buffered status + body to the real ResponseWriter.
func (w *bufferWriter) flush() {
	w.ResponseWriter.WriteHeader(w.Status())
	if w.buf.Len() > 0 {
		_, _ = w.ResponseWriter.Write(w.buf.Bytes())
	}
}

// etagOf computes the strong, quoted, content-based ETag of a body.
func etagOf(body []byte) string {
	sum := sha256.Sum256(body)
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

// ifNoneMatch reports whether the client's If-None-Match header matches etag
// (supports the comma-separated list form and "*").
func ifNoneMatch(header, etag string) bool {
	if header == "" {
		return false
	}
	for _, tok := range strings.Split(header, ",") {
		tok = strings.TrimSpace(tok)
		tok = strings.TrimPrefix(tok, "W/")
		if tok == "*" || tok == etag {
			return true
		}
	}
	return false
}

// Cache is the read-path caching middleware for a public, cacheable GET class.
// It serves fresh LRU hits (honoring If-None-Match → 304), otherwise runs the
// handler, and — only for a 200 — stores the body with a strong ETag and sets
// Cache-Control per the payload class. Non-200 responses are passed through
// unstored with no-store (errors are never cached; caching doc §2). maxAge is
// the Cache-Control max-age and doubles as the LRU TTL (they coincide by class).
func Cache(store *ResponseCache, m *metrics.Metrics, clk clock.Clock, maxAge time.Duration) gin.HandlerFunc {
	cacheControl := "public, max-age=" + itoaSeconds(maxAge)
	return func(c *gin.Context) {
		route := c.FullPath()
		key := cacheKey(c, "public")

		if e, ok := store.get(key); ok {
			if m != nil {
				m.CacheHits.WithLabelValues(route).Inc()
			}
			c.Header("ETag", e.etag)
			c.Header("Cache-Control", cacheControl)
			if ifNoneMatch(c.GetHeader("If-None-Match"), e.etag) {
				c.Status(http.StatusNotModified)
				c.Abort()
				return
			}
			c.Data(http.StatusOK, e.contentType, e.body)
			c.Abort()
			return
		}
		if m != nil {
			m.CacheMisses.WithLabelValues(route).Inc()
		}

		bw := &bufferWriter{ResponseWriter: c.Writer}
		c.Writer = bw
		c.Next()
		c.Writer = bw.ResponseWriter // restore before flushing

		body := bw.buf.Bytes()
		status := bw.Status()
		if status != http.StatusOK {
			// Errors / non-200: never cached, never conditional (caching §2).
			c.Header("Cache-Control", "no-store")
			bw.flush()
			return
		}

		etag := etagOf(body)
		contentType := bw.ResponseWriter.Header().Get("Content-Type")
		if contentType == "" {
			contentType = "application/json; charset=utf-8"
		}
		store.put(&cacheEntry{
			key: key, body: append([]byte(nil), body...), etag: etag,
			contentType: contentType, expiresAt: clk.Now().Add(maxAge),
		})

		hdr := bw.ResponseWriter.Header()
		hdr.Set("ETag", etag)
		hdr.Set("Cache-Control", cacheControl)
		if ifNoneMatch(c.GetHeader("If-None-Match"), etag) {
			bw.ResponseWriter.WriteHeader(http.StatusNotModified)
			return
		}
		bw.flush()
	}
}

// itoaSeconds renders a duration's whole seconds without importing strconv at
// call sites.
func itoaSeconds(d time.Duration) string {
	secs := int64(d / time.Second)
	if secs <= 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for secs > 0 {
		i--
		b[i] = byte('0' + secs%10)
		secs /= 10
	}
	return string(b[i:])
}
