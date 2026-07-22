# ForecastIQ — UI State Contracts

**Version**: 1.0 (UI ↔ Backend Reconciliation Board)
**Status**: Authoritative — binding for frontend implementation and API design
**Inputs**: `docs/ui/00-screen-inventory.md` §3; `docs/ui/02-ui-design-specification.md` §13; `docs/product/05-business-rules.md` BR-FRESH; `docs/api/00-api-requirements.md` §2, §6; `docs/reviews/03-ui-backend-conflicts.md`

This document is the contract between backend response shapes and frontend state rendering. Every state has exactly one triggering backend representation; every backend signal has exactly one UI treatment. Partial data is a **normal** state, never reduced to a generic 500.

---

## 1. UI State → Backend Representation Matrix

| UI state | Triggering condition | API mechanism | Response code | Response field(s) | Retryable | User message (binding copy) | Monitoring signal |
|----------|---------------------|---------------|---------------|-------------------|-----------|----------------------------|-------------------|
| Loading | Fetch in flight | — (client) | — | — | — | (skeleton; `aria-busy="true"`) | — |
| No data (location has none) | Location exists; zero collections/rankings | Success with empty data + metadata | 200 | `data.rankings: []`, `location.created_at`, `collection_started_at` | no (resolves with time) | "Collecting since {date} — first data appears within ~1 h. Rankings require ≥7 days of matched data." + progress hint | `locations_without_collections` gauge |
| No locations exist | `GET /locations` → `[]` | Success with empty array | 200 | `data: []` | no | "No locations monitored yet." (+ admin CTA) | n/a (bootstrap state) |
| Insufficient sample | Cell below threshold | Success; per-cell status | 200 | `ranking_status: "unranked"`, `reason`, `sample_count`, `min_sample_count` | no | "Insufficient data ({n}/{threshold} samples) — collecting continues." | `unranked_cells` gauge |
| Provisional | 10–29 samples or coverage [0.5, 0.8) | Success; per-cell status | 200 | `ranking_status: "provisionally_ranked"`, `sample_count`, `coverage` | no | "Provisional — based on {n} samples ({threshold} required for full ranking)." | n/a |
| Delayed | Freshness threshold exceeded (level 1) | Success; freshness block | 200 | `freshness.state: "delayed"`, `freshness.last_updated`, `age_seconds`, `threshold_seconds` | yes (refetch) | Badge: "Data delayed" + last-updated | `freshness_delayed` counter |
| Stale | Freshness threshold exceeded (level 2) | Success; freshness block | 200 | `freshness.state: "stale"`, same fields | yes (refetch) | Banner: "⚠ Data may be out of date — last updated {relative} ({absolute local time})" (persistent, non-dismissible) | `freshness_stale` counter + operator alert |
| Unavailable (data) | No successful collection/observation in 24h | Success; freshness block | 200 | `freshness.state: "unavailable"`, `reason` | yes | "No data available — {reason if known}." | `freshness_unavailable` counter + operator alert |
| Partial (provider failure) | Some providers unservable, others OK | **HTTP 200** with available data (API req §6) | 200 | `warnings: [{provider_id, code: "provider_unavailable"\|"stale", message}]`; affected entries omitted from list, present in warnings | yes | "{Provider} data temporarily unavailable — showing {n} of {m} providers." + per-row badge | `partial_responses_total{endpoint}` counter |
| Observation unavailable | No observation source data for location/period | Success; explicit flag | 200 | `observations_available: false`, `observation_provenance.note` | no | "Ground truth unavailable for this period — accuracy metrics not computed." + provenance note | `observation_gaps_total` counter |
| Unauthorized | No/invalid/expired token | Error envelope | 401 | `type: …/unauthorized`, `request_id` | yes (re-auth) | "Sign in to access this page." + [Sign in] | `auth_failures_total` counter |
| Forbidden | Valid auth, insufficient role | Error envelope | 403 | `type: …/forbidden`, `request_id` | no | "Administrator access required. This section is restricted to platform operators." + sign-in-switch hint | `forbidden_total` counter |
| Not found | Unknown resource/route | Error envelope | 404 | `type: …/not_found` | no | "Page not found." + Overview link | n/a |
| Validation failure | Invalid input on mutation | Error envelope + field errors | 422 | `type: …/validation`, `errors[{field, message}]` | yes (correct input) | Inline field errors + `role="alert"` summary | n/a |
| Conflict (duplicate) | BR-LOC-01 dedup / idempotency collision | Error envelope | 409 | `type: …/duplicate` or `…/conflict`, existing-resource reference | yes (override/adjust) | "Possible duplicate of '{name}' ({distance} away)." + [View existing] [Add anyway] | n/a |
| Rate limited | Per-key/IP budget exhausted | Error envelope + headers | 429 | `type: …/rate_limited`, `Retry-After`, `X-RateLimit-*` | yes (after Retry-After) | "Too many requests — retry in {n}s." | `rate_limited_total` counter |
| Provider dependency failure (server-side) | Upstream provider error during a **mutation-like** admin action (trigger/replay) | Error envelope | 502 | `type: …/provider_unavailable`, `retryable: true` | yes | "Provider unreachable — circuit {state}. Next automatic probe in {n}s." | `provider_errors_total{provider}` |
| Failed (full API failure) | Network error / 5xx on all fetches | — (no response) | 5xx/timeout | — | yes (3 attempts max) | "Unable to load data. The server may be temporarily unavailable." + [Retry]; after 3 failures: "Still unable to connect. Check your network or try again later." (retry removed) | `api_5xx_total`, uptime check |
| Disconnected client | `navigator.onLine=false` or all fetches network-error | — (client detection) | — | — | auto on reconnect | Banner: "You appear to be offline. Showing cached data from {time}." Mutations disabled + tooltip "Requires network connection." On restore: "Connection restored." → auto-refetch | n/a (client-side) |
| Timeout | Fetch exceeded deadline | — (client abort) | — | — | yes | Treated as full failure (above) with retry | `api_timeout_total` |

## 2. State Priority (When Multiple Apply)

Binding render order (doc 02 §13.8, ratified):

1. **Offline** (network-level; overrides all)
2. **Permission denied** (access-level)
3. **Full error** (server-level)
4. **Stale** (data-level; banner shown WITH available data)
5. **Partial failure** (provider-level; badges shown WITH partial data)
6. **Empty / Insufficient** (data-level)
7. **Loading** (transitional)

Composition rule: states 4–6 compose (stale banner + partial badge + insufficient rows simultaneously). States 1–3 are exclusive at screen level (but cached data may render beneath the offline/error overlay, dimmed and labeled).

## 3. Freshness Contract (BR-FRESH binding)

Server computes freshness per data type; UI never derives state from timestamps alone (BR-FRESH-02).

| Data type | fresh | delayed | stale | unavailable |
|-----------|-------|---------|-------|-------------|
| Forecast collection (per provider-location) | < 75 min | 75–180 min | > 180 min | no success 24h or circuit open |
| Observations (per location) | < 90 min | 90–240 min | > 240 min | none in 24h |
| Rankings (per cell) | recompute < 2h from latest input | 2–6h | > 6h | inputs unavailable |
| Operational health (admin) | < 5 min | 5–15 min | > 15 min | health endpoint down |

Freshness block shape (all time-sensitive payloads):
```json
"freshness": {"state": "fresh|delayed|stale|unavailable", "last_updated": "…Z", "age_seconds": 720, "threshold_seconds": 4500}
```
`age_seconds` enables relative display; `threshold_seconds` enables honest "why stale" tooltips. Stale data is always served with its staleness (BR-FRESH-01) — never silently as current.

## 4. Partial-Result Contract (API req §6, ratified and extended)

- Transport: **HTTP 200** (not 206 — pragmatic convention for browser/cache/client compatibility; 206 has poor client support and cache semantics).
- `warnings[]` is top-level, never nested per data row.
- Affected providers: **omitted from data lists, present in `warnings[]`** with `provider_id`, `code` (`provider_unavailable` | `stale`), human-readable `message`.
- Unaffected providers render normally — the UI must not degrade the whole screen.
- `warnings[]` absent or empty = complete response.
- Every screen maps each warning code to its badge treatment (§1, "Partial" row).

## 5. Cached Data Contract (client-side)

When a fetch fails and previously loaded data exists:
- Data shown at 50% opacity with label "Cached — last loaded {relative} ({absolute})".
- Cached data is never shown without the label (PC-10: when something is broken, the UI says so).
- Cache is per-route in-memory (React Query/SWR default); no persistent cache in MVP (stale-persistent-cache risk outweighs benefit).

## 6. Loading Contract

- Skeleton matches final layout exactly (CLS < 0.1, AC-6.3).
- Skeleton debounce: shown only if fetch > 100ms (prevents flash on fast responses).
- Refetch: previous data at 50% opacity + skeleton overlay; no layout shift.
- `aria-busy="true"` on loading regions; pulse disabled under `prefers-reduced-motion`.

## 7. Offline Contract

- Detection: `navigator.onLine` + fetch network-error classification (TypeError/AbortError with no response).
- Banner: full-width below header, `role="alert"`, `aria-live="assertive"`.
- Mutations disabled with explanatory tooltip (Export CSV, Retry, all admin actions).
- Recovery: on `online` event → "Connection restored." → auto-refetch all active queries → banner removed after success.

## 8. Mutation Error Recovery Contract

| Mutation class | UI behaviour | Retry behaviour |
|----------------|--------------|-----------------|
| Idempotent POST (locations, api-keys, exports, trigger) with Idempotency-Key | Pessimistic (wait for response) | Safe to retry with same key (24h window returns original result) |
| PUT (configurations, locations) | Pessimistic | Natural idempotency (full replace) |
| DELETE (keys, users, me) | Pessimistic + confirmation dialog | Not auto-retried; user-initiated only |
| Replay | Pessimistic + confirm | Server-side idempotent (snapshot dedup); UI may retry safely |

Optimistic UI: **only** `PATCH /me` preferences (low-risk, self-scoped). All other mutations pessimistic.

## 9. State → Screen Applicability

| State | S-01 | S-02 | S-03 | S-04 | S-05 | S-06 | S-09 | S-10 | S-11–14 |
|-------|------|------|------|------|------|------|------|------|---------|
| Loading | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| No locations | ✓ | — | — | ✓ | ✓ | — | — | — | ✓ (S-12) |
| No data yet | ✓ | ✓ | ✓ | ✓ | ✓ | — | — | — | — |
| Insufficient | ✓ | ✓ | ✓ | ✓ | — | — | — | — | — |
| Partial | ✓ | ✓ | ✓ | ✓ | ✓ | — | — | ✓ | — |
| Observation unavailable | ✓ (context line) | ✓ | — | ✓ | ✓ | — | — | ✓ (collector) | — |
| Stale | ✓ | ✓ | ✓ | ✓ | ✓ | — | — | ✓ | — |
| 401/403 | — | — | — | — | — | — | ✓ | ✓ | ✓ |
| Validation | — | — | — | — | — | — | ✓ | — | ✓ |
| Rate limited | any (banner) | any | any | any | any | — | ✓ | ✓ | ✓ |
| Full error | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Offline | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
