# ForecastIQ — WP-25 Security Hardening: Delivery Review Board

**Review date**: 2026-07-27
**Work package**: WP-25 — Security Hardening (PR #31, `feature/wp25-security-hardening`)
**Reviewed SHA**: `8123635` (post ADR-033 merge-up)
**Decision**: **REJECTED — 2 High + 4 Medium; textually merged with ADR-033 but semantically pre-ADR-033**

---

## 1. Context

No Critical findings: the body limit correctly uses `http.MaxBytesReader` (not
Content-Length alone — the chunked bypass is absent), and CORS fails closed (no
wildcard, no origin reflection, no credentialed wildcard). The middleware code
is sound. The package fails because the ADR-033 merge brought in the Caddyfile
deletion **without reconciling the WP-25 artifacts** that assumed a Caddy +
systemd + VPS topology — leaving security obligations owned by nobody.

## 2. Findings

### High

**DRB-WP25-001 (H)** — HSTS is set by no layer. `security.go` comments name
`deploy/caddy/Caddyfile` as "the primary enforcement point" and defer HSTS to
"the TLS terminator (Caddy)" — but Caddy was deleted by ADR-033, and this repo's
Terraform manages Cloudflare **DNS only** (no `cloudflare_zone_settings_override`,
no HSTS). Verified: `security.go:11-23` references the deleted file; no HSTS
anywhere. Fix: enable HSTS at Cloudflare via Terraform (zone settings) and
update the comment to name Cloudflare, or set it app-side gated on a prod flag.

**DRB-WP25-002 (H)** — `rotation-drill.sh` targets the dead topology and doesn't
validate rotation: live-mode step 1 is `systemctl restart forecastiq`
(no systemd unit under ADR-033 — should be `docker compose ... up -d
--force-recreate app`; note env_file changes need recreation, not restart); the
`owner=root`/`600` secrets-file assertions encode the systemd `EnvironmentFile`
model and contradict the compose `env_file` read by the `deploy` user (if true,
every deploy EACCESses); and `--live` only *prints* five manual steps and exits —
it never restarts, never verifies the service returned with rotated creds.
Verified: `rotation-drill.sh:46,109`.

### Medium

**DRB-WP25-003 (M)** — "CORS final allowlist" not delivered: nothing sets
`FIQ_CORS_ALLOW_ORIGINS` for production (not in docker-compose.prod.yml), so the
API defaults to `http://localhost:3000` while the dashboard is at
`https://app.<domain>`. Fail-closed (dashboard breaks, not a vuln) but the scope
item + threat-model §15 "config in repo" are unmet. Add the prod origin to the
compose file (it's not a secret).

**DRB-WP25-004 (M)** — 413 mapping is partly fictional: the comment references
`MapBodyLimitError` which **does not exist**; the real helper `IsBodyTooLarge`
is called only from the test file. Production handlers bind via
`ShouldBindJSON` → a chunked oversize body returns **400**, not 413, yet
`TestRequestBodyLimit_StreamedOverflow` asserts 413 against a test-local handler
(a "test that doesn't test"). Verified: `IsBodyTooLarge` has no non-test caller.
The security property holds; the contract/test claim does not. Wire the helper
into the shared binding-error path + add a router-level test, or correct the
comment/test to the real 400 contract.

**DRB-WP25-005 (M)** — Threat-matrix coverage ~5 of 16 threats against a "100%
covered" acceptance bar, and no matrix artifact (threat → test mapping) was
produced. Genuinely covered: DoS/body limit (good negative + off-by-one +
chunked), rate-limit 429 + Retry-After, CORS (positive **and** negative), panic
recovery. Header tests are unit-level only — nothing asserts `NewRouter` wires
the middleware in the claimed order. Add the matrix doc + one router-level chain
test.

**DRB-WP25-006 (M)** — Dashboard CSP owned by nobody: threat model §2 relies on
"CSP on dashboard"; there is no `web/public/_headers` and a Next static export
can't emit headers itself. API-side CSP omission is defensible; the dashboard
half of "security headers" is unaddressed post-ADR-033. Add a Cloudflare Pages
`_headers` file.

### Low

**DRB-WP25-007 (L)** — `RequestBodyLimit(1<<20)` hardcoded in the router while
`config.RequestBodyLimit` is defined but consumed nowhere (dead config; future
tuning silently no-ops). Thread the config value through.
**DRB-WP25-008 (L)** — declared-oversize 413 uses `AbortWithStatus` (empty body),
breaking the `respond.Error` JSON envelope contract every other error path uses.

## 3. Verified correct

`http.MaxBytesReader` used (no Content-Length-only bypass); CORS negative case
tested and fails closed; off-by-one exact-limit test present; `X-XSS-Protection:
0` is the correct modern value; `set -euo pipefail`, no secret values echoed.

## 4. Rate-limit note (scope item, Info→tracked)

Per-IP limiting keys on `c.ClientIP()`; under ADR-033 all traffic arrives from
Cloudflare egress IPs → effectively one shared global bucket until the
`CF-Connecting-IP` trusted-proxy mapping lands (already flagged as a follow-up in
ADR-033). With the origin on plain `:80`, restricting the EC2 security group to
Cloudflare IP ranges is also the only thing preventing direct-origin
`X-Forwarded-For` spoofing against Gin's default trust-all-proxies. WP-25 was the
WP meant to tune rate limiting; this is unaddressed.

## 5. Decision

**REJECTED.** Reconcile the artifacts with ADR-033: assign every header an owner
(001, 006 — Cloudflare zone settings / Pages `_headers` / app middleware),
rewrite the rotation drill for the Docker topology and make it actually restart
+ verify (002), deliver the production CORS allowlist (003), make the 413
contract real or correct the tests (004), produce the threat matrix + a
router-level test (005), and tune rate limiting for the CF-egress-IP reality (or
document the security-group + CF-Connecting-IP plan). 007/008 fixed or tracked.
Re-review requires green CI on the new SHA.
