# ForecastIQ — Error & Partial-Result Contracts

**Version**: 1.0 (UI ↔ Backend Reconciliation Board)
**Status**: Authoritative — extends `docs/api/00-api-requirements.md` §2; binding error taxonomy for UI mapping
**Companion**: `docs/ui/06-ui-state-contracts.md` (frontend rendering of each error class)

---

## 1. Standard Error Envelope (RFC 7807 + extensions, ratified)

```json
{
  "type": "https://forecastiq.example/errors/validation",
  "title": "Validation Error",
  "status": 422,
  "detail": "Field 'latitude' must be between -90 and 90",
  "instance": "/api/v1/locations",
  "request_id": "0d1c…",
  "retryable": false,
  "docs": "https://forecastiq.example/docs/errors#validation",
  "errors": [{"field": "latitude", "message": "must be between -90 and 90"}]
}
```

Extensions beyond RFC 7807 (all optional except `request_id`):
- `request_id` — always present (correlation, NFR-OBS02; displayed on S-15 500 page).
- `retryable` — boolean guidance for clients (drives Retry-button logic in state contracts).
- `docs` — stable documentation anchor per error type.
- `errors[]` — field-level detail on validation failures only.

## 2. Error Taxonomy (complete, with UI mapping)

| `type` suffix | HTTP | Trigger | `retryable` | UI state (state-contracts ref) | Notes |
|---------------|------|---------|-------------|-------------------------------|-------|
| `validation` | 422 | Invalid input shape/range/enum | true (after correction) | Validation failure | `errors[]` required |
| `unauthorized` | 401 | Missing/invalid/expired JWT or API key | true (re-auth) | Unauthorized | Generic message; no account-existence hints |
| `forbidden` | 403 | Valid auth, insufficient role/scope | false | Forbidden | Never leak resource existence |
| `not_found` | 404 | Unknown route/resource | false | Not found | Same shape for missing + forbidden-on-private resources (no enumeration) |
| `conflict` | 409 | State conflict (circuit open, self-lockout, one-active-export) | varies | Contextual (tooltip/dialog) | `detail` explains resolution path |
| `duplicate` | 409 | BR-LOC-01 near-duplicate; idempotency-key collision with different body | true (override/adjust) | Conflict (duplicate) | Includes `existing_resource: {id, name, distance_degrees}` for dedup case |
| `rate_limited` | 429 | Per-key or per-IP budget exhausted | true (after `Retry-After`) | Rate limited | `Retry-After` header + `X-RateLimit-*` headers |
| `provider_unavailable` | 502 | Upstream provider failure during admin trigger/replay | true | Provider dependency failure | `detail` includes circuit state + next probe time; never raw provider error body |
| `payload_unavailable` | 422 | Replay requested but payload expired/corrupt | false | Replay blocked | `detail`: retention (90d) or corruption notice |
| `internal` | 500 | Unhandled failure | true (limited) | Full error | **Sanitized**: no stack trace, no SQL, no provider internals; request_id only |
| `service_unavailable` | 503 | Graceful degradation exhausted (DB unreachable, no cacheable state) | true | Full error | `Retry-After` when known (e.g., startup drain) |

### 2.1 Idempotency collisions (API req §1)

- Same `Idempotency-Key`, same body, within 24h → original response replayed (same status + body). Not an error.
- Same key, **different** body → `duplicate` 409 with `detail: "Idempotency-Key already used with different request body"`.

### 2.2 Security constraints on errors (Security Architect, binding)

- 401 vs. 403 distinction preserved internally but auth-failure messages never distinguish "user exists" from "user unknown" (anti-enumeration).
- 404 on workspace-scoped resources returns 404 (not 403) for non-members once Level 3 RLS lands; MVP single-workspace: 403 for role failures, 404 for unknown IDs.
- Validation messages describe the constraint, never the stored value ("must be between -90 and 90", not "differs from stored 1.4927").
- Provider error details (HTTP bodies from OpenWeather etc.) are **never** forwarded to clients — classified into `error_code` taxonomy internally (FC-13), surfaced operationally, summarized as `provider_unavailable` publicly.
- `request_id` is the only internal correlation surface exposed.

## 3. Partial-Result Contract (complete specification)

### 3.1 When partial applies

Partial results occur when a multi-provider payload can serve some providers but not others. Causes: provider circuit open, provider stale beyond threshold, provider collection gap for the requested period.

### 3.2 Transport decision

**HTTP 200** (not 206). Rationale (board decision, ratifies API req §6):
- 206 Partial Content is defined for range requests on single resources; clients (fetch, axios, caches) handle it inconsistently for JSON APIs.
- 207 Multi-Status (WebDAV) adds unfamiliar semantics and poor intermediary support.
- 200 + `warnings[]` + `partial_result: true` composes cleanly with ETags, CDNs, and the stale-cache degradation path (NFR-A07).

### 3.3 Shape

```json
{
  "data": {"rankings": [ /* only servable providers */ ]},
  "partial_result": true,
  "warnings": [
    {"provider_id": "…", "code": "provider_unavailable", "message": "OpenWeather data temporarily unavailable", "since": "…Z"},
    {"provider_id": "…", "code": "stale", "message": "Visual Crossing data last updated 5h ago", "since": "…Z"}
  ]
}
```

### 3.4 Rules

1. Affected providers omitted from data arrays; always present in `warnings[]`.
2. `warnings[]` codes are a closed enum: `provider_unavailable`, `stale`. New codes require API governance review (additive within v1).
3. `partial_result` is redundant with `warnings[]` non-emptiness — kept for explicit client checks and future extension.
4. Rankings with partial providers: unaffected providers keep their ranks computed from the last complete evaluation batch (rankings are batch-computed, so a live provider outage does not reshuffle ranks mid-batch — statistical stability). The warning communicates the lag.
5. All-providers-failed is **not** partial: serve last cached batch with `freshness.state = stale` + warning, or 503 if no cache exists.
6. UI renders per state-contracts §1 "Partial" row: unaffected providers normal; affected get badge + banner; never a broken row.

### 3.5 Retry guidance per warning code

| Code | Client behaviour |
|------|------------------|
| `provider_unavailable` | No immediate retry value (next collection cycle is recovery); refetch on user navigation or 5-min ambient refresh |
| `stale` | Refetch on interaction; staleness banner persists until fresh (BR-FRESH-01) |

## 4. Error → Screen Behaviour Summary

Every screen maps every error class to exactly one behaviour (full matrix in `docs/ui/06-ui-state-contracts.md` §1). No screen shows a raw error body. No screen reduces a partial failure to a full-error state. No mutation proceeds without server-side validation regardless of client-side checks (NFR-SEC05).
