# ForecastIQ — Caching and Partial Results (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: `docs/api/02-response-conventions.md` §3, §6; `docs/api/03-error-and-partial-result-contracts.md`; NFR-A07

---

## 1. Cache Architecture (no Redis — constraints §3)

```text
Request → middleware (auth, rate limit)
  → cache lookup: key = sha256(route + sorted params + auth-class)
     HIT + fresh → ETag check → 304 or body (metric: lru_cache_hits_total)
     MISS → handler → DB → serialize → store (body + etag, TTL per class) → respond
```

| Component | Specification |
|-----------|---------------|
| Store | In-process LRU (hashicorp/golang-lru or equivalent) |
| Capacity | 256 entries (≈ 20 MB worst case at 80 KB max response) |
| TTL | Per response class (§2); expired entries evicted lazily |
| Key | Route template + canonical sorted query params + auth class (public / user / admin — never per-user for public endpoints) |
| ETag | Strong, SHA-256 of serialized body; generated at store time |
| Invalidation | TTL expiry only (batch writes refresh data within 60 s of computation; no explicit busting needed) |
| Restart behaviour | Cold cache (acceptable; repopulates in seconds at MVP traffic) — Redis promotion solves this |
| Scope exclusion | Admin endpoints and mutations: never cached (`no-store`) |

**Why this is sufficient at MVP:** rankings/accuracy reads hit pre-computed rows (≤ 10⁵ total); DB p95 < 100 ms cold; LRU absorbs 60 s polling from ≤ 100 concurrent users → effective DB load < 20 qps. Measured trigger for Redis: p95 > 150 ms dominated by these reads (constraints §4).

## 2. Cache Header Classes (binding, conventions §6)

| Payload class | Cache-Control | ETag | LRU TTL |
|---------------|---------------|------|---------|
| Rankings / accuracy summaries / methodology | `public, max-age=60` | strong | 60 s |
| Forecast-comparison (past dates) | `public, max-age=300` | strong | 300 s |
| Forecast-comparison (today) | `public, max-age=60` | strong | 60 s |
| Locations / providers | `public, max-age=300` | strong | 300 s |
| Trends (/accuracy) | `public, max-age=60` | strong | 60 s |
| Admin endpoints | `no-store` | none | — |
| Mutations | `no-store` | none | — |
| Errors (all) | `no-store` | none | — |

Conditional GET: `If-None-Match` honored on all cacheable GETs → 304 with ETag echo (empty body). Admin health polling (60 s, ≤ 2 operators): `no-store` + cheap assembly (< 200 ms) — 304 optimization unnecessary (implementation choice per conventions §6 note).

## 3. Stale-Cache Degradation (NFR-A07)

When the DB is transiently unavailable:

```text
Handler DB error
  → expired LRU entry exists?
     YES → serve it with freshness.state forced to "stale" + warning
           {code: "stale", message: "Data may be out of date (served from cache)"}
     NO  → 503 service_unavailable (RFC 7807) + Retry-After if known
```

- Stale serving applies ONLY to public cacheable classes (never admin, never mutations).
- Staleness is always explicit (BR-FRESH-01: never silently served as current).
- Mutations during DB outage: 503 (no queueing, no write-behind — honesty over availability).

## 4. Partial-Result Implementation (contracts §3, binding)

### 4.1 Assembly algorithm

```text
requested_providers = param OR all active providers
FOR each provider:
  state = evaluate(provider):
    circuit open                     → unavailable
    last success > stale threshold   → stale
    collection gap for period        → unavailable (for period-scoped payloads)
    else                             → servable
servable   → include in data arrays
unavailable → warnings[] {provider_id, code: "provider_unavailable", message, since}
stale       → include data + warnings[] {code: "stale", message, since}
```

### 4.2 Rules (fixed, tested)

1. Transport: HTTP 200 (never 206/207 — rationale in contracts §3.2).
2. Affected providers omitted from data arrays, always present in warnings[].
3. `partial_result: true` ⇔ warnings[] non-empty.
4. Warning codes closed enum: `provider_unavailable`, `stale` (governance for additions).
5. Rankings during outage: last complete batch rows (no mid-batch reshuffling — statistical stability); warning communicates lag.
6. All-providers-failed: NOT partial → stale-cache path (§3) or 503.
7. UI rendering per state contracts (unaffected providers normal; affected badged).

### 4.3 Retry guidance (echoed in docs, not headers)

| Code | Client behaviour |
|------|------------------|
| provider_unavailable | No immediate retry value; refetch on navigation / 5-min ambient refresh |
| stale | Refetch on interaction; banner persists until fresh |

## 5. Response-Size Governance

| Endpoint | Max size | Enforcement |
|----------|----------|-------------|
| /rankings | 16 KB | Bounded rows (providers ≤ 10) |
| /accuracy/summary | 40 KB | Bounded cells |
| /accuracy | 80 KB | ≤ 365 buckets × providers; limit param |
| /forecast-comparison | 20 KB | 24 h × bounded series |
| Raw lists | limit ≤ 200 rows | Pagination param cap |
| Admin health | < 30 KB | Bounded cells + sections |

Oversized-by-design requests (e.g., 365 d daily × all providers) still fit documented maxima — verified by integration test asserting response byte size.

## 6. Client-Side Export Interaction

CSV exports are client-generated from the current (bounded) view data — no server export path except GDPR (`POST /me/export`). Export reflects current filters; metadata header per conventions §5. This removes an entire class of server load/abuse (C-18 decision).

## 7. Cross-Reference

- Conventions (normative shapes): `docs/api/02-response-conventions.md`
- Error taxonomy: `docs/api/03-error-and-partial-result-contracts.md`
- Redis promotion: `docs/architecture/10-scaling-and-evolution.md` §1.1
