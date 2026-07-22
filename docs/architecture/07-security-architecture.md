# ForecastIQ — Security Architecture (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: NFR-SEC01..16; `docs/security/01-ui-authorization-matrix.md`; ADR-008; error contracts §2.2

Companion documents: `docs/security/02-threat-model.md` (full threat model), `docs/security/03-data-classification.md`, `docs/security/04-secrets-management.md`, `docs/security/05-audit-requirements.md`.

---

## 1. Authentication Architecture (ADR-008 implementation)

| Concern | Specification |
|---------|---------------|
| Provider | Supabase Auth (managed) |
| Flows supported | Email+password registration (mandatory email verification), login, password reset (email), password update, session refresh, logout |
| Token verification | RS256 JWT via JWKS (Supabase endpoint); JWKS cached 15 min with rotation tolerance (fetch on unknown kid, rate-limited); claims verified: issuer, audience, expiry, subject |
| Session model | Supabase access token (≤ 1 h) held by dashboard; API is stateless (no server sessions); refresh rotation with theft detection (managed); concurrent sessions allowed (NFR-SEC16) |
| User provisioning | On first authenticated API call: upsert `users` row from token claims (auth_subject, email); role defaults `user`; first account seeded `admin` at bootstrap via migration seed |
| External identity mapping | `users.auth_subject` = Supabase user id (unique); portable to other OIDC providers (ADR-008 migration path) |
| Workspace creation | System workspace exists at bootstrap; users join implicitly (MVP single workspace) |
| Logout | Dashboard revokes refresh token via Supabase; API stateless (access token valid until expiry — acceptable at ≤ 1 h) |
| Account disabling | Admin PATCH status → local 401 on next request (auth middleware checks user status) + Supabase Admin API ban_user |
| Email verification | Mandatory (Supabase config); unverified accounts cannot obtain valid sessions |
| Password reset | Entirely Supabase-managed; app never sees passwords |
| Local profile | email (synced), role, status, default_location_id, preferences — app-owned |
| Audit | All auth events (AUTH-07): login, failed login, registration, verification, reset, key lifecycle |

**Backend authorization is independent of frontend visibility** (binding): every endpoint declares its auth requirement; middleware enforces before handler execution; frontend route guards are UX only.

## 2. Authorization Architecture

### 2.1 Model: Role-based with object-level checks

| Layer | Mechanism | What it enforces |
|-------|-----------|------------------|
| Middleware (Gin) | `RequireAuth()`, `RequireRole(admin)`, `RequireScope(x)` | Authentication, role gates, API key scopes |
| Use-case layer | Object-level checks (key owner = principal; self-lockout guards) | Ownership, business rules |
| Repository layer | Query scoping (WHERE user_id = $principal for key listings) | Data filtering |
| Database | No RLS in MVP (single workspace; additive at Level 3 per ADR-009) | — |

### 2.2 Permission Summary (full matrix: `docs/security/01-ui-authorization-matrix.md`)

| Resource | Public | User | Admin |
|----------|--------|------|-------|
| Rankings/accuracy/providers/locations (read) | ✓ | ✓ | ✓ |
| Raw forecasts/observations (read) | — | ✓ (key-scoped) | ✓ |
| Own profile/keys/preferences | — | ✓ (self only) | ✓ (self) |
| Locations/providers/configurations (write) | — | — | ✓ |
| Collection trigger/replay/recompute | — | — | ✓ |
| User management | — | — | ✓ (self-lockout guarded) |
| Audit events (read) | — | — | ✓ |
| Raw payload files | Never served (no route exists); admin sees key + checksum prefix only | | |

### 2.3 Anti-patterns explicitly prevented

- No route-level-only role checks where object ownership matters (key management checks owner).
- No resource enumeration: 404 for unknown IDs; auth failures never distinguish user existence.
- No provider error forwarding: upstream bodies classified internally, never echoed.
- No credential exposure: `credential_ref` absent from all API structs (serializer-level, not runtime filter).

## 3. Threat Model Summary (full: `docs/security/02-threat-model.md`)

| Threat | Primary mitigation | Detection |
|--------|--------------------|-----------|
| Account takeover | Managed auth (rotation, brute-force defense); email verification; status check on every request | Failed-login audit; Supabase alerts |
| Token theft | Short-lived tokens; refresh rotation with reuse detection; no token in logs | Rotation anomaly → family revocation |
| Broken object-level auth (BOLA) | Object checks in use cases; repository scoping; single workspace eliminates cross-tenant surface | Audit trail; integration tests per endpoint |
| Provider API key leakage | Env-only secrets; credential_ref indirection; never logged/returned; repo secret scanning | Key-use alerts from provider dashboard |
| Raw payload exposure | No file-serving route; volume not web-accessible; admin sees key prefix only | Filesystem permissions audit |
| Injection (SQL/command) | Parameterized queries only (no string SQL); no shell-outs; input validation middleware | golangci-lint + code review |
| SSRF via provider URLs | Provider base URLs are seeded config (not user input); no user-supplied URLs fetched | Config change audit |
| Denial of service | Per-key + per-IP rate limiting; request size limits (1 MB); bounded queries (required filters, limits) | Rate-limit metrics; 429 alerts |
| Brute-force login | Supabase-managed + app-level limiter on auth-adjacent endpoints | Managed service + audit |
| Export abuse | Client-side CSV from bounded views only; GDPR export: one active job (409), 24 h expiry, unguessable link | Job audit |
| Log leakage | Structured logging allowlist (no tokens, keys, payloads, emails beyond subject ref) | Log review checklist |
| Dependency compromise | govulncheck + Trivy in CI; Dependabot; minimal base image (distroless) | CI failures block merge |
| Malicious provider payload | Schema validation; range validation; size limits; JSON-only parsing; checksum before parse | schema_drift alerts |
| CSV formula injection | Client-generated exports prefix formula-leading cells (defense: `#` comment header + no formula-starting values in data — weather numerics only; documented) | Export template review |
| Insecure CORS | Explicit origin allowlist; no wildcards; preflight cached | Config in repo (reviewed) |
| Secrets in repo/CI | Secret scanning (gitleaks in CI); .env gitignored; no secrets in images | CI gate |

## 4. Platform Security Controls

| Control | Implementation |
|---------|----------------|
| TLS in transit | TLS 1.3 everywhere (Caddy → app is localhost only; DB connection TLS-enforced by managed provider) |
| Encryption at rest | Managed DB disk encryption (vendor); volume: Hetzner encrypted volume option (enabled) |
| Secure headers | Caddy: HSTS (1y, includeSubDomains), X-Content-Type-Options, X-Frame-Options DENY, Referrer-Policy, CSP on dashboard (Pages config) |
| Input validation | Middleware: JSON schema validation per endpoint (OpenAPI-derived); range/enum/format checks; reject unknown fields on mutations |
| Request size limits | 1 MB body limit (Caddy + app); query parameter count bounded |
| Dependency scanning | govulncheck (Go), npm audit (dashboard), Trivy (image) — CI gates |
| Container scanning | Trivy on distroless image in CI |
| Audit logging | All auth + admin actions → immutable `audit_events` (1 y) |
| Rate limiting | In-process token bucket: per-key (60/min default), per-IP public bucket, auth-adjacent stricter (10/min); Redis promotion for multi-instance |
| Data classification | Four tiers (public / internal / confidential / restricted) — `docs/security/03-data-classification.md` |

## 5. Database Security

- Single application credential (no superuser); scoped to forecastiq schema; cannot CREATE/DROP outside migrations role.
- Migrations applied by a separate credential (deploy pipeline only) with DDL rights.
- Immutability triggers owned by a role the app credential cannot alter.
- Connection TLS enforced; managed DB network-restricted (IP allowlist: VPS IP + CI runner via temporary access where needed).

## 6. Cross-Reference

- Threat model: `docs/security/02-threat-model.md`
- Data classification: `docs/security/03-data-classification.md`
- Secrets: `docs/security/04-secrets-management.md`
- Audit: `docs/security/05-audit-requirements.md`
- API auth detail: `docs/api/07-authentication-and-authorization.md`
- ADRs: ADR-008 (auth), ADR-009 (workspace/RLS path)
