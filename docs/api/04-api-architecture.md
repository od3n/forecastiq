# ForecastIQ — API Architecture (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: `docs/api/00-api-requirements.md`; `docs/api/01-screen-api-contracts.md`; `docs/api/02-response-conventions.md`; `docs/api/03-error-and-partial-result-contracts.md` (all authoritative; this document consolidates implementation architecture)

---

## 1. Composition Strategy (binding, reconciliation board)

**Reusable domain endpoints as the backbone + two purpose-built screen endpoints + no BFF.**

| Strategy | Verdict | Rationale |
|----------|---------|-----------|
| Reusable domain endpoints (rankings, accuracy, locations, providers, forecasts, observations) | **Backbone** | Stable, cacheable per-endpoint, serve multiple screens |
| Purpose-built: `GET /forecast-comparison` | **Approved (C-19)** | Public S-05 needs bounded public payload while raw endpoints stay user+ |
| Purpose-built: extended `GET /accuracy/summary` (provider mode + collection_window) | **Approved (C-08)** | Eliminates N+1 grid fan-out |
| BFF aggregation layer | **Rejected** | Couples backend releases to dashboard layout; defeats ETag caching; extra layer a 1–2 engineer team should not maintain |
| Endpoint-per-card | **Rejected** | Proliferation; the board capped at max 2 requests per screen load |
| One enormous endpoint | **Rejected** | Cache granularity loss; over-fetching |

Dashboard composes ≤ 2 requests per screen load (verified per screen in doc 01 §11).

## 2. Conventions Summary (implementation view)

| Concern | Implementation |
|---------|----------------|
| Versioning | URL path `/api/v1/`; additive changes within v1; breaking → v2 with ≥ 6-month Sunset window (API-10) |
| Route naming | Plural nouns; nested only for true ownership (`/admin/collections/{id}/replay`); no verbs in paths except action subresources (replay, trigger, recompute, export) |
| Auth | `Authorization: Bearer <jwt>` OR `X-API-Key`; public set per AUTH-08; middleware-declared per route |
| Workspace scoping | Single system workspace; ownership joins present (Level 3 RLS additive); no workspace path segment in v1 |
| Pagination | Cursor (keyset) + `limit` 1..200; `pagination: {next_cursor, has_more}`; no total_count |
| Filtering | Whitelisted query params per endpoint (OpenAPI enum); unknown params → 422 |
| Sorting | `sort=±field` on whitelisted fields; default documented per endpoint |
| Date/time | ISO 8601 UTC `Z` everywhere; date params (`date=2026-07-21`) interpreted in location tz where documented (FvA) |
| Units | Explicit suffixes (`_c`, `_mm`, `_ms`, `_hpa`, `_pct`) + `metadata.units` block |
| Errors | RFC 7807 + extensions (request_id, retryable, docs, errors[]); 11-class taxonomy (error contracts §2) |
| Partial results | HTTP 200 + `warnings[]` + `partial_result` (contracts §3) |
| Freshness | Server-computed block on all time-sensitive payloads (conventions §2) |
| Provenance | Block on derived payloads + per-entity fields (conventions §4) |
| Methodology metadata | `methodology_version` + `weights_version` on all derived payloads |
| Request IDs | `X-Request-Id` validated (UUID format) or generated; echoed in response + logs |
| Correlation | Job/collection IDs in admin payloads; request_id is the only public correlation surface |
| Idempotency keys | `Idempotency-Key` on mutable POSTs; 24 h store; per-principal scope; collision → 409 `duplicate` |
| Rate-limit headers | `X-RateLimit-Limit/Remaining/Reset` on every response; 429 + Retry-After |
| Cache headers | Per-class table (conventions §6); ETag strong content-based |
| Conditional GET | `If-None-Match` → 304 on all cacheable GETs |
| CORS | Allowlist (production origin + localhost:*); headers include Authorization, X-API-Key, Idempotency-Key, X-Request-Id; preflight 1 h |
| Response limits | Max sizes per endpoint (16–80 KB documented); request body 1 MB |

## 3. Handler Architecture

```text
Gin router
  → middleware chain: Recovery → RequestID → Logger → RateLimit → Auth(optional/required/role)
    → Validate (OpenAPI-derived schema)
      → Handler (thin: parse → use-case call → envelope assembly)
        → Module use case (business logic, tx)
```

- Handlers contain **no business logic** (testable use cases own it).
- Envelope assembly is a shared package: data + metadata + freshness + provenance + attribution + warnings + pagination (conventions §1 applicability rules — include only meaningful fields, never null placeholders).
- ETag: SHA-256 of serialized body (strong); LRU stores body + etag keyed by (route, params, auth-class).
- Rounding applied at serialization (methodology §5: ratios 4 dp, temp/rain 2 dp, wind 1 dp, pressure 2 dp).

## 4. Freshness Computation (server-side, BR-FRESH-02)

Per data type, computed from row timestamps at response assembly:

| Type | Source | fresh → delayed → stale → unavailable |
|------|--------|---------------------------------------|
| Forecast collection (per cell) | MAX(completed_at) success | 75 / 180 min / 24 h or circuit open |
| Observations (per location) | MAX(observed_at) | 90 / 240 min / 24 h |
| Rankings (per cell) | MAX(calculated_at) vs latest input | 2 h / 6 h / inputs unavailable |
| Operational health | Assembly time | 5 / 15 min / endpoint down |

Response carries one top-level `freshness` block; per-provider differences → `warnings[]` (code `stale`).

## 5. Partial-Result Assembly

Handler-level: query each provider in the requested set; servable → data array; unservable (circuit open / stale beyond threshold / gap) → warnings[] entry `{provider_id, code, message, since}`; affected omitted from data (fixed choice). All-failed → stale-cache path (NFR-A07) or 503. Rankings during outage: last batch rows (stable), warning communicates lag.

## 6. API Change Governance (API §7, CI-enforced)

1. OpenAPI generated from code annotations (oapi-codegen or swag) — spec drift impossible.
2. CI: `openapi-diff` against main — breaking changes fail build.
3. Additive changes allowed within v1 (new optional fields/endpoints).
4. Deprecation: `Deprecation: true` + `Sunset` headers; usage tracked per endpoint via RED metrics labels.

## 7. Cross-Reference

- Endpoint catalog: `docs/api/05-endpoint-catalog.md`
- OpenAPI outline: `docs/api/06-openapi-outline.yaml`
- Auth architecture: `docs/api/07-authentication-and-authorization.md`
- Caching: `docs/api/08-caching-and-partial-results.md`
