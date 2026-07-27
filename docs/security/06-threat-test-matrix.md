# ForecastIQ — Threat-Model Test Matrix (WP-25)

**Reference**: `docs/security/02-threat-model.md` (16 mandated threat areas §1–§16).
**Purpose**: the WP-25 acceptance artifact — every threat maps to its mitigation
and the test (or CI gate) that exercises it. "Architectural" entries are
mitigations enforced by design/absence of the attack surface rather than a
dedicated test.

| # | Threat | Primary mitigation | Test / gate |
|---|--------|--------------------|-------------|
| 1 | Account takeover | Supabase auth; app verifies JWT (JWKS), no local password store | `adapters/auth/jwks/jwks_test.go`; `internal/identity/credential_test.go` |
| 2 | Token theft | Short-lived JWT verified per request; dashboard CSP + security headers | `adapters/auth/jwks/jwks_test.go`; `web/public/_headers` (CSP); `internal/api/security_test.go` (headers) |
| 3 | BOLA | Workspace/ownership scoping + role guards on every resource route | `test/integration/authz_matrix_test.go` |
| 4 | Provider API key leakage | `credential_ref` indirection; log sanitizer redacts key-shaped fields | `internal/platform/logging/sanitize_test.go`; `internal/identity/credential_test.go` |
| 5 | Raw payload exposure | Payload store access gated; replay via authorized handlers only | `adapters/payloadstore/filesystem_test.go`; `test/integration` (payload replay authz) |
| 6 | Injection (SQL/cmd/template) | pgx parameterized queries throughout; no shell/template exec on input | Architectural — no string-built SQL (depguard + review); covered indirectly by integration suite |
| 7 | SSRF via provider URLs | Provider registry pins fixed base URLs; no user-supplied fetch targets | Architectural — `internal/collection` registry (no dynamic URLs) |
| 8 | Denial of service | 1 MB body limit (`MaxBytesReader`) + per-key rate limit (429 + Retry-After) | `internal/api/security_test.go` (`TestRequestBodyLimit_*`, `TestRateLimit_Returns429`) |
| 9 | Brute-force login | Delegated to Supabase (lockout/throttle at the IdP) | Architectural — IdP-owned (ADR-008); app has no password path |
| 10 | Export abuse | One in-progress export per user; expiring download window | `test/integration/export_test.go` |
| 11 | Log leakage | Sanitizing slog handler; recovery emits no stack/SQL | `internal/platform/logging/sanitize_test.go`; `internal/api/security_test.go` (`TestRecovery_NoStackTrace`) |
| 12 | Dependency compromise | govulncheck (PR + nightly), Trivy vuln+secret image scan, Dependabot | CI: `backend-checks` govulncheck, `image` trivy, `scheduled.yml` nightly |
| 13 | Malicious provider payload | Response size cap (`MaxResponseBytes`) + strict decode/validation | `adapters/observationsources/openmeteo/*_test.go`; collection validation tests |
| 14 | CSV formula injection | Export utility neutralizes `= + - @ TAB CR` leading chars | `web/test/csv.test.ts` |
| 15 | Insecure CORS | Explicit origin allowlist; fails closed (no wildcard/reflection) | `internal/api/security_test.go` (`TestCORS_RejectsUnknownOrigin`, `TestCORS_AllowsConfiguredOrigin`) |
| 16 | Secrets in repo / CI logs | gitleaks PR gate + weekly history scan; rotation-drill repo grep | CI: `security` gitleaks, `scheduled.yml` weekly; `deploy/scripts/rotation-drill.sh` |

## Header ownership (ADR-033)

TLS terminates at Cloudflare; the origin serves plain HTTP :80 (no Caddy).

| Header | Owner |
|--------|-------|
| HSTS, Always-Use-HTTPS | Cloudflare zone settings (`terraform/cloudflare.tf`) |
| API response headers (nosniff, frame-deny, referrer, permissions, CSP) | app `SecurityHeaders` middleware |
| Dashboard CSP + headers | Cloudflare Pages `web/public/_headers` |

## Known residuals (tracked)

- **Rate limiting behind Cloudflare**: `ClientIP()` sees Cloudflare egress IPs
  until a `CF-Connecting-IP` trusted-proxy mapping lands; the EC2 security group
  must restrict :80 to Cloudflare ranges to prevent direct-origin
  `X-Forwarded-For` spoofing (ADR-033 §4 follow-up).
- **Dashboard CSP** must be validated in-browser against the deployed Pages
  build (connect-src/style-src) before launch.
