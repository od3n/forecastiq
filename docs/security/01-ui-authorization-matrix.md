# ForecastIQ — UI Authorization Matrix

**Version**: 1.0 (UI ↔ Backend Reconciliation Board)
**Status**: Authoritative — binding for backend middleware and frontend role gating
**Inputs**: AUTH-01..09; `docs/domain/01-domain-model.md` §8; `docs/ui/02-ui-design-specification.md` §2.5; security reconciliation (board mandate)
**Principle**: Hiding controls in the frontend is UX only. **Every protected action is authorized server-side.** A UI action without a server-side rule is a defect.

---

## 1. Roles (MVP)

| Role | Definition | Source |
|------|-----------|--------|
| Public | Unauthenticated visitor | No token |
| User | Authenticated, `users.role = 'user'` | Supabase JWT → `users.auth_subject` |
| Admin | Authenticated, `users.role = 'admin'` (operator) | Same; first account seeded admin at bootstrap |

Level 3 RBAC (organization roles, workspace membership) is deferred; the matrix is structured to extend (columns, not code changes).

## 2. Screen Access Matrix

| Screen | Public | User | Admin | Server enforcement |
|--------|--------|------|-------|--------------------|
| S-01 Overview | ✓ view | ✓ view | ✓ view | Public endpoint; no auth required (AUTH-08) |
| S-02 Location Detail | ✓ view | ✓ | ✓ | Public |
| S-03 Provider Detail | ✓ view | ✓ | ✓ | Public |
| S-04 Trends | ✓ view | ✓ | ✓ | Public |
| S-05 Forecast vs. Actual | ✓ view | ✓ | ✓ | Public via `/forecast-comparison` (C-19); raw endpoints user+ |
| S-06 Methodology | ✓ view | ✓ | ✓ | Public |
| S-07 Onboarding | — | ✓ own | ✓ own | Bearer; self-scope |
| S-08 Auth | ✓ use | — | — | Supabase-managed |
| S-09 Settings | — | ✓ own | ✓ own | Bearer; object-level: user_id = token subject |
| S-10 Admin Health | — | — | ✓ | Bearer + role=admin (middleware) |
| S-11 Admin Providers | — | — | ✓ | Same |
| S-12 Admin Locations | — | — | ✓ | Same |
| S-13 Admin Schedules | — | — | ✓ | Same |
| S-14 Admin Users & Audit | — | — | ✓ | Same |
| S-15 Error pages | ✓ | ✓ | ✓ | n/a |

403 handling: admin endpoints return the `forbidden` error class; UI renders the permission-denied state (state contracts §1). 401 (no/invalid token) renders the sign-in prompt. The two are never conflated.

## 3. Action Authorization Matrix (board-mandated columns)

| Persona/role | Screen | View | Create | Update | Deactivate | Delete | Export | Manage providers | Manage users | Rationale |
|--------------|--------|------|--------|--------|------------|--------|--------|------------------|--------------|-----------|
| Public | S-01..S-06 | ✓ (read-only) | — | — | — | — | ✓ CSV (client-side, public data + attribution) | — | — | AUTH-08 portfolio visibility; exports contain only public derived data with attribution (BR-ATTR-01, BR-LIC-01 gate) |
| Public | S-05 raw data | — | — | — | — | — | — | — | — | Raw `/forecasts`/`/observations` user+ (AUTH-08) — bulk surface gated |
| User | S-07/S-09 profile | ✓ own | ✓ own API keys | ✓ own preferences/keys | — | ✓ own keys; ✓ own account (self-delete) | ✓ own GDPR export | — | — | Self-service scope; object-level check user_id = subject |
| User | Raw data API | ✓ (user+ endpoints, own key scopes) | — | — | — | — | ✓ via API (key-scoped) | — | — | BR-05 key scopes + per-key rate limit |
| User | Admin screens | ✗ (403) | — | — | — | — | — | — | — | Role gate |
| Admin | S-10 Health | ✓ | ✓ trigger collection | — | — | — | — | — | — | Operational recovery (C-10); audited |
| Admin | S-11 Providers | ✓ | — (providers seeded/config-level) | ✓ configurations (schedule, attribution, credential_ref) | ✓ provider status | — (never delete) | — | ✓ | — | ADMIN-01; BR-LOC-03 pattern (disable ≠ delete); credentials never displayed (BR-08) |
| Admin | S-12 Locations | ✓ | ✓ POST /locations (Idempotency-Key, dedup 409) | ✓ name/timezone/status | ✓ status patch | — (soft only) | — | — | — | ADMIN-02; BR-LOC-01; immutability of historical data requires soft-delete |
| Admin | S-13 Schedules | ✓ | — | ✓ schedule via configuration PUT | — | — | — | — | — | ADMIN-04; FC-07 next-cycle apply |
| Admin | S-13 Replay/Recompute | ✓ | ✓ replay (new collection) / ✓ recompute (new rows) | — | — | — | — | — | — | ADMIN-06; FC-14; immutability preserved (never mutates originals) |
| Admin | S-14 Users | ✓ list | — (self-registration only) | ✓ status (disable/enable) | ✓ disable | ✓ delete (AUTH-09 scope) | ✓ admin-triggered user export | — | ✓ | ADMIN-05; self-lockout guard (409 on self disable/delete); Supabase propagation server-side |
| Admin | S-14 Audit | ✓ read | — | — | — | — (immutable) | — | — | — | Audit rows immutable (NFR-SEC11); no update/delete path exists |

## 4. Object-Level Authorization Rules

| Resource | Rule | Enforcement point |
|----------|------|-------------------|
| API keys | Owner-only (user_id = subject) OR admin | Repository query scoped + middleware |
| User profile/preferences | Self-only | Middleware subject check |
| GDPR export jobs | Self OR admin-on-target | `requested_by` / target check |
| Locations/providers/configurations | Admin for writes; public for reads of non-sensitive fields | Middleware role check |
| Provider credentials (`credential_ref`) | Never returned by any endpoint; status only ("Configured"/"Not set") | Serializer exclusion (BR-08) — defense in depth: field absent from API structs, not filtered at runtime |
| Raw payloads (`raw_payload_object_key`) | Never exposed publicly; admin sees key + checksum prefix only (no download path in MVP) | Admin serializer; no file-serving route |
| Audit events | Admin read-only | Middleware |
| Observations/forecasts (raw) | user+ with valid key/session; scope per key | Auth middleware + key-scope check |

## 5. Workspace Isolation (MVP → Level 3)

- MVP: single system workspace; all ownership-bearing rows carry `workspace_id = system` (domain §8.2). Authorization queries include the workspace join (one indexed join, free at this scale) so Level 3 RLS is additive.
- Level 3: PostgreSQL RLS keyed on `workspace_id = current_setting('app.workspace_id')`; child-table ownership derived via parent joins (documented denormalization tradeoff, domain §8.2).
- No UI exposes cross-workspace data in MVP (nothing to cross).

## 6. Rate Limiting and Abuse Controls (UI-relevant)

| Surface | Limit | UI signal |
|---------|-------|-----------|
| Public endpoints (per IP) | Shared public bucket (default 60 req/min equivalent) | 429 state → "Too many requests — retry in {n}s" |
| Per API key | 60 req/min default, per-key configurable (AUTH-05) | `X-RateLimit-*` headers; 429 state |
| Auth-adjacent (sign-in, reset) | Supabase-managed + app-level limiter (AUTH-04) | Generic refusal copy (no enumeration) |
| Admin trigger endpoint | Provider rate-budget check before dispatch | 409/429 with budget reset time |

## 7. Audit Coverage (every UI mutation)

| Action | Audit event `action` | Details captured |
|--------|---------------------|------------------|
| Login / failed login / registration / verification / reset | `auth.*` | subject, IP |
| Key create/revoke | `api_key.create/revoke` | key_prefix, scopes |
| Preferences/default-location update | `user.preferences_updated` | changed fields (not values for sensitive) |
| GDPR export request/completion | `export.requested/completed` | target user |
| Account delete (self/admin) | `user.deleted` | actor, target, scope summary |
| Provider status/config change | `provider.status_changed/config_updated` | old/new status, schedule diff |
| Location create/update/disable | `location.*` | fields, dedup override flag |
| Collection trigger | `collection.triggered` | provider, location, actor |
| Replay | `collection.replayed` | source collection id, new collection id |
| Recompute | `rankings.recompute_triggered` | scope filters |
| User disable/delete (admin) | `admin.user_disabled/deleted` | actor, target |

All audit rows: immutable, 1-year retention, `user_id` + `ip_address` + `resource_type/id` (domain model AUDIT_EVENT).

## 8. Security Reconciliation Findings (resolved)

| # | Finding | Resolution |
|---|---------|------------|
| SEC-01 | Screen inventory mapped S-03 to a "public health subset" of an admin endpoint | Fixed: `/accuracy/summary` extension (C-08); no operational data public |
| SEC-02 | Doc 03 exposed API latency/retry counts publicly | Removed (C-15) |
| SEC-03 | Public S-05 required user+ endpoints | Bounded public `/forecast-comparison` (C-19); raw endpoints stay gated |
| SEC-04 | Admin user management had no endpoints (risk of Supabase-dashboard-only management = unaudited) | Four audited endpoints (C-09) |
| SEC-05 | Replay button on failed collections without payloads | Correct action semantics (C-10); replay gated on payload existence (422 `payload_unavailable`) |
| SEC-06 | Error messages could leak provider internals | Error taxonomy binding: provider bodies never forwarded (error contracts §2.2) |
| SEC-07 | Frontend role gating alone would be insufficient | Every action has a server rule (§3); 403/401 contracts defined |
| SEC-08 | Credential display risk in S-11 edit dialog | Status-only indicator; serializer-level exclusion (BR-08) |
| SEC-09 | Account enumeration via auth flows | Generic messages; reset-sent regardless of existence |
| SEC-10 | Self-lockout (sole admin disables self) | 409 guard on self disable/delete |
