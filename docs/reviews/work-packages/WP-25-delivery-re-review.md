# ForecastIQ — WP-25 Security Hardening: DRB Confirmatory Re-Review

**Review date**: 2026-07-27
**Work package**: WP-25 — Security Hardening (PR #31, `feature/wp25-security-hardening`)
**Prior review**: WP-25-delivery-review.md — REJECTED on `8123635` (DRB-WP25-001…008)
**Reviewed SHA**: `c7399b4`
**Decision**: **ACCEPTED**

---

## 1. Verification of evidence

| Check | Result |
|-------|--------|
| Commit identity: local branch == remote == CI head | ✅ all `c7399b4` |
| Seven jobs green (backend-checks/integration/migrations/api-contract/security/image/frontend-checks) | ✅ first run |
| `go test -race ./internal/api/...` | ✅ |
| `terraform validate` (with new zone-settings resource) | ✅ |
| Web build ships `out/_headers` | ✅ (1041 bytes, CSP present) |

## 2. Finding closure

| Finding | Status | Resolution |
|---------|--------|-----------|
| 001 (H) HSTS owned by nobody | ✅ | `cloudflare_zone_settings_override` sets HSTS (1y, includeSubDomains, preload) + always_use_https; `security.go` comment rewritten to name Cloudflare and explain why app-side HSTS is inert on a plain-HTTP origin |
| 002 (H) rotation drill on dead topology | ✅ | Rewritten for Docker: `--live` now `docker compose up -d --force-recreate app` (env_file needs recreation) + readyz + smoke verify; secrets-file check is now `owner=deploy` matching the compose env_file model |
| 003 (M) CORS allowlist absent | ✅ | `FIQ_CORS_ALLOW_ORIGINS` set in docker-compose.prod.yml (dashboard origin; overridable via `FIQ_DASHBOARD_ORIGIN`) |
| 004 (M) 413 mapping fictional | ✅ | `respond.Classify` now `errors.As`-matches `*http.MaxBytesError` → 413 through the shared path; test rewritten to exercise `respond.Error` (production path), asserts problem+json; dead `MapBodyLimitError` reference removed |
| 005 (M) no threat matrix | ✅ | `docs/security/06-threat-test-matrix.md` maps all 16 threats → test/gate/architectural + header-ownership table + tracked residuals |
| 006 (M) dashboard CSP unowned | ✅ | `web/public/_headers` (Cloudflare Pages) with CSP + headers; ships in `out/` (build-verified) |
| 007 (L) dead body-limit config | ✅ | `RouterConfig.BodyLimit` threaded from `cfg.RequestBodyLimit` |
| 008 (L) 413 empty body | ✅ | Declared-oversize now returns the problem+json envelope; regression test added |

## 3. Tracked residuals (documented, not blocking)

- Rate limiting keys on `ClientIP()` = Cloudflare egress IPs until the
  `CF-Connecting-IP` mapping + EC2 security-group restriction to Cloudflare
  ranges land (ADR-033 §4 follow-up; recorded in the matrix).
- `ssl = flexible` at Cloudflare (origin is plain HTTP :80); the CF→origin hop
  relies on the security-group restriction above. Move to `strict` if the
  origin ever terminates TLS.
- Dashboard CSP must be validated in-browser against the deployed Pages build
  (connect-src/style-src) before launch — noted in `_headers` and the matrix.

## 4. Decision

**ACCEPTED.** All 8 findings closed; seven jobs green on `c7399b4`; the header
ownership question the first review raised now has an explicit owner per header
(Cloudflare zone / Pages `_headers` / app middleware), and the 413 contract is
real and tested. Residuals are documented and tied to the ADR-033 §4 follow-up.
PR #31 ready to merge. **WP-26 (Performance Validation) becomes eligible.**
