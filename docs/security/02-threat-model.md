# ForecastIQ — Threat Model (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: `docs/architecture/07-security-architecture.md` §3 (summary); NFR-SEC01..16; OWASP Top 10 (NFR-SEC14)

Method: asset-centric lightweight threat model (STRIDE-informed) covering the 16 mandated threat areas. Each entry: asset, actor, attack path, mitigation, detection, residual risk.

---

## 1. Account Takeover

| Attribute | Assessment |
|-----------|------------|
| Asset | User accounts (admin especially) |
| Actor | External attacker |
| Path | Credential stuffing; password reset interception; session hijack |
| Mitigation | Supabase-managed: breach-list checks, min 12 chars (NFR-SEC15), email verification mandatory, refresh rotation with reuse detection (theft → family revocation), brute-force rate limiting (managed + app-level on auth-adjacent, AUTH-04); admin disable effective immediately (role from DB, not JWT) |
| Detection | `auth.login_failed` audit + Supabase alerts; anomalous subject activity |
| Residual | Low — password lifecycle entirely vendor-managed; no local credential surface |

## 2. Token Theft

| Attribute | Assessment |
|-----------|------------|
| Asset | Supabase JWTs (access ≤ 1 h) |
| Actor | XSS attacker; network observer; malicious browser extension |
| Path | Exfiltrate token from browser storage; replay |
| Mitigation | TLS 1.3 everywhere; short-lived access tokens; refresh rotation detects replay; tokens never logged; API stateless (no server-side token store to breach); CSP on dashboard limits XSS |
| Detection | Rotation anomaly (Supabase); token use after disable → 401 |
| Residual | Medium (browser-side exposure inherent to SPAs) — bounded by 1 h lifetime + rotation |

## 3. Broken Object-Level Authorization (BOLA)

| Attribute | Assessment |
|-----------|------------|
| Asset | Other users' keys/profiles; admin resources |
| Actor | Authenticated user |
| Path | Enumerate IDs; access /api-keys/{other}; admin endpoints with user role |
| Mitigation | Object checks in use cases (key owner = principal); role middleware on all admin routes; UUIDv7 non-guessable (time-ordered but 128-bit); 404 for unknown (no existence leaks); single workspace eliminates cross-tenant surface in MVP |
| Detection | Audit trail on all mutations; integration tests per endpoint (authorization matrix as test cases) |
| Residual | Low |

## 4. Provider API Key Leakage

| Attribute | Assessment |
|-----------|------------|
| Asset | OpenWeather API key |
| Actor | Repo reader; log reader; API consumer |
| Path | Committed secret; logged value; returned by config endpoint |
| Mitigation | Env-only (systemd EnvironmentFile 0600); `credential_ref` stores the env key NAME, never the value; serializer-level field exclusion (BR-08 — absent from structs, not filtered at runtime); gitleaks CI gate; never logged (logging allowlist) |
| Detection | gitleaks on all commits + history scan; provider dashboard usage anomalies |
| Residual | Low — rotation runbook < 30 min |

## 5. Raw Payload Exposure

| Attribute | Assessment |
|-----------|------------|
| Asset | Stored provider responses (licensing-sensitive) |
| Actor | External; authenticated user |
| Path | Direct file access; path traversal via API |
| Mitigation | **No file-serving route exists** (attack surface absent); volume not web-accessible (Caddy proxies only /api); admin sees key + checksum prefix only; filesystem perms (app user only); ToS-gated retention (90 d) |
| Detection | Filesystem audit; no download endpoint to monitor (by design) |
| Residual | Low (VPS root compromise is the only path — covered by platform hardening) |

## 6. Injection (SQL / Command / Template)

| Attribute | Assessment |
|-----------|------------|
| Asset | Database; VPS |
| Actor | External attacker |
| Path | SQLi via params; command injection via inputs |
| Mitigation | pgx parameterized queries only (no string SQL anywhere — lint rule); no shell-outs in application; no template rendering of user input; input validation middleware (types, ranges, enums); JSON-only bodies |
| Detection | golangci-lint (govet/sqlclosecheck); 422 validation errors monitored |
| Residual | Low |

## 7. SSRF via Provider URLs

| Attribute | Assessment |
|-----------|------------|
| Asset | VPS network (internal services, metadata endpoints) |
| Actor | External (if URL were user-controlled) |
| Path | User-supplied URL fetched server-side |
| Mitigation | **No user-supplied URLs are ever fetched** — provider base URLs are seeded configuration (migration-seeded, admin-only change, audited); no webhook receivers in MVP |
| Detection | Config change audit (`provider.config_updated`) |
| Residual | Negligible (path architecturally absent) |

## 8. Denial of Service

| Attribute | Assessment |
|-----------|------------|
| Asset | API availability |
| Actor | External |
| Path | Request flood; expensive-query abuse; large bodies |
| Mitigation | Per-IP public bucket + per-key limits (60/min default) with 429 + Retry-After; 1 MB body limit; all list queries bounded (required filters + limit ≤ 200 + keyset pagination); pre-computed aggregate reads (no expensive on-demand aggregation); Cloudflare edge absorbs volumetric |
| Detection | 429 rate metrics; latency SLO burn alerts; uptime checks |
| Residual | Medium (single VPS ceiling — honest 99.5% target; volumetric mitigated by edge) |

## 9. Brute-Force Login

| Attribute | Assessment |
|-----------|------------|
| Asset | Accounts |
| Mitigation | Supabase-managed rate limiting + lockout; app-level limiter on auth-adjacent endpoints (10/min/IP); generic error messages (no account-existence hints, anti-enumeration) |
| Detection | `auth.login_failed` audit; managed service alerts |
| Residual | Low |

## 10. Export Abuse

| Attribute | Assessment |
|-----------|------------|
| Asset | GDPR export files; CSV data |
| Actor | Authenticated user |
| Path | Export spam; scraping via export; formula injection via CSV |
| Mitigation | One active GDPR job per user (409 partial-unique index); files 24 h expiry + unguessable UUID path; deleted after expiry; CSV is **client-generated** from bounded public views only (no server data-exfil path); CSV formula injection: data cells are weather numerics/ISO strings only (no formula-leading characters possible), documented; attribution header in every export (licensing) |
| Detection | `export.requested/completed` audit |
| Residual | Low |

## 11. Log Leakage

| Attribute | Assessment |
|-----------|------------|
| Asset | Tokens, keys, PII in log pipeline |
| Mitigation | Structured logging allowlist (fields explicitly added; no whole-object dumps); prohibited: tokens, credential values, provider bodies, emails (subject refs only); code review checklist; log sample review quarterly |
| Detection | Periodic grep audits of log stream for known secret patterns |
| Residual | Low |

## 12. Dependency Compromise

| Attribute | Assessment |
|-----------|------------|
| Asset | Application integrity |
| Mitigation | govulncheck + npm audit + Trivy image scan in CI (blocking); Dependabot; minimal deps (Go stdlib-heavy); distroless runtime image; go.sum/npm lockfile committed |
| Detection | CI failures; Dependabot alerts |
| Residual | Medium (supply-chain inherent; mitigated by scanning + minimal surface) |

## 13. Malicious Provider Payload

| Attribute | Assessment |
|-----------|------------|
| Asset | Collection pipeline integrity |
| Path | Compromised/MITM provider response with hostile content |
| Mitigation | TLS to providers; 10 MB response cap; JSON-only parsing (typed structs, no eval); schema validation rejects unexpected shapes; physical range validation (absurd values → suspect, never trusted); checksum computed before parse; payloads never executed/rendered (stored bytes only) |
| Detection | schema_drift alerts; suspect-value metrics |
| Residual | Low |

## 14. CSV Formula Injection

Covered in §10. Additional binding rule: if future export types include free-text fields, cells starting with `= + - @ \t \r` must be single-quote-prefixed (implemented at that time; not needed for numeric weather exports).

## 15. Insecure CORS

| Mitigation | Explicit origin allowlist (production dashboard + localhost dev); no `*`; credentials only for allowlisted origins; preflight cached 1 h; config in repo (reviewed) |
| Residual | Low |

## 16. Secrets in Repository / CI Logs

| Mitigation | gitleaks on every push + scheduled history scan; `.env*` gitignored; CI never echoes secrets (masked by GitHub Actions); deploy via SSH key (Actions secret); image builds contain no env files (verified by Trivy secret scan) |
| Detection | gitleaks alerts; Actions secret masking |
| Residual | Low |

---

## 17. Threat Model Governance

- Review triggers: new endpoint class, new external dependency, new data class, any incident, annual (NFR-SEC14 OWASP checklist).
- Each mitigation above maps to a test (security test matrix in `docs/testing/02-testing-strategy.md`).
- Material residual-risk changes → risk register update + ADR if architectural.
