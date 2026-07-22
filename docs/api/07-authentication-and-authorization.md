# ForecastIQ — API Authentication and Authorization (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: AUTH-01..09; ADR-008; `docs/security/01-ui-authorization-matrix.md`; `docs/architecture/07-security-architecture.md`

---

## 1. Authentication Flows

### 1.1 Session authentication (dashboard users)

```mermaid
sequenceDiagram
    participant B as Browser
    participant SB as Supabase Auth
    participant API as ForecastIQ API

    B->>SB: register/login (email+password)
    SB-->>B: access JWT (≤1h) + refresh token
    B->>API: GET /rankings (no auth — public)
    B->>API: GET /me (Authorization: Bearer <jwt>)
    API->>API: verify JWT via JWKS (cached 15min)
    API->>API: upsert users row (provision on first use)
    API->>API: check users.status != disabled
    API-->>B: 200 profile
    Note over B,SB: refresh rotation managed by Supabase SDK;<br/>reuse detection revokes family
```

### 1.2 API key authentication (programmatic)

- `X-API-Key: fiq_...` → prefix lookup → argon2id hash verify → check revoked_at/expires_at → principal = key's user with key's scopes.
- Key shown once at creation; stored hashed only; `last_used_at` updated (throttled write: at most once per minute per key).

### 1.3 JWT Verification Specification

| Step | Detail |
|------|--------|
| JWKS source | Supabase project JWKS endpoint; cached 15 min; unknown kid → one refetch (rate-limited 1/min) |
| Algorithm | RS256 only (reject `alg: none`, HS256) |
| Claims | iss = project issuer; aud = authenticated; exp enforced; sub = auth_subject |
| Failure modes | Expired → 401 `unauthorized` (retryable via re-auth); invalid signature → 401; JWKS down + cache miss → 401 retryable (never fail-open) |
| Clock skew | 30 s leeway |

## 2. Authorization Model

### 2.1 Middleware chain (per route, declarative)

```go
r.GET("/rankings", h.Rankings)                                    // public
r.GET("/me", RequireAuth(), h.Me)                                 // any authenticated
r.DELETE("/api-keys/:id", RequireAuth(), h.RevokeKey)             // + object check in use case
r.GET("/admin/health", RequireAuth(), RequireRole("admin"), h.Health)
```

| Middleware | Enforces |
|-----------|----------|
| `RequireAuth()` | Valid Bearer JWT or API key; principal loaded; user status active |
| `RequireRole(role)` | principal.Role matches (from users table, not JWT claim — admin disable takes effect immediately) |
| `RequireScope(scope)` | API-key principal has scope (JWT sessions have full user scope) |

### 2.2 Object-level checks (use-case layer)

| Resource | Check |
|----------|-------|
| API key revoke/list | key.user_id = principal.id (or admin) |
| PATCH /me | self only (subject match) |
| Export jobs | requested_by = principal OR admin targeting user |
| Admin user disable/delete | target ≠ self (409 self-lockout guard) |
| Location/provider writes | role = admin (no finer granularity in MVP) |

### 2.3 Public set (AUTH-08, binding)

Public read: `/rankings`, `/rankings/methodology`, `/accuracy`, `/accuracy/summary`, `/forecast-comparison`, `/providers`, `/locations` (read), health endpoints. Rationale: portfolio visibility; derived data with attribution. Gated: raw `/forecasts` + `/observations` (user+), all admin, all mutations except self-service.

## 3. Role Model (MVP)

| Role | Capabilities | Assignment |
|------|-------------|-----------|
| public | Read public endpoints | None |
| user | public + raw data (key-scoped) + self-service (profile, keys, export, delete) | Default on registration |
| admin | user + all admin operations | First account seeded at bootstrap; subsequent: operation-level only (no endpoint — documented Level 3 RBAC gap, D-04) |

Role read from `users.role` on every request (not cached in JWT) — admin disable is immediately effective.

## 4. API Key Scopes (MVP set)

| Scope | Grants |
|-------|--------|
| `read:public` | Public endpoints (default, implicit) |
| `read:data` | `/forecasts`, `/observations` (raw) |
| `read:analysis` | All analysis endpoints (already public; scope exists for future gating) |

Per-key rate limit (default 60/min, configurable at creation, max 300/min). Scopes stored as JSONB; additive scope values need no migration.

## 5. Session and Token Lifecycle

| Event | Behaviour |
|-------|-----------|
| Login | Supabase issues access (≤ 1 h) + refresh; app audit `auth.login` |
| Refresh | Rotation with theft detection (reuse → family revocation, NFR-SEC16) |
| Logout | SDK revokes refresh; API stateless (access valid until expiry ≤ 1 h — accepted) |
| Admin disable | users.status = disabled → next API call 401; Supabase ban_user (service-role) |
| Account delete | AUTH-09 flow; Supabase admin delete; local rows removed; audit retained (user_id NULL) |
| Failed login | Supabase-side rate limiting + app audit `auth.login_failed` (no enumeration hints) |

## 6. Supabase Admin API Usage (backend only)

| Operation | When | Guard |
|-----------|------|-------|
| ban_user | Admin disable | Service-role key; failure → 502, local unchanged |
| delete_user | Account deletion | After local deletion succeeds + audit written |
| (never) list/export users | — | Not used; our users table is authoritative for profile data |

Service-role key: env-only, never in dashboard bundle (verified by build-time grep CI check).

## 7. Audit Integration

Every auth event + every authorized mutation writes `audit_events` in the same transaction (identity module consumes audit.Recorder). Action registry per authorization matrix §7.

## 8. Cross-Reference

- Security architecture: `docs/architecture/07-security-architecture.md`
- Full matrices: `docs/security/01-ui-authorization-matrix.md`
- ADR-008 (Supabase decision + migration trigger)
