# ForecastIQ — WP-23 CI/CD + Deployment: Delivery Review Board

**Review date**: 2026-07-26
**Work package**: WP-23 — CI/CD and Deployment (PR #29, `feature/wp23-cicd-deployment`)
**Reviewed SHA**: `4c3d111` (post WP-22 merge-up)
**Decision**: **REJECTED — 4 Critical + 5 High findings; 3 scope items absent**

---

## 1. Verification of evidence

| Check | Result |
|-------|--------|
| Branch merged with `main` (contains accepted WP-22) | ✅ `4c3d111` |
| Six PR jobs green on pre-merge SHA `4f44305` | ✅ (run re-verification required on new SHA) |
| WP-22 dependency accepted | ✅ (WP-22-delivery-re-review.md) |

Rejection is on content: the deploy pipeline **cannot complete a single successful
deploy end-to-end** — WP-23's acceptance test is literally "deploy pipeline works",
and four independent Critical defects each break the happy path. Key findings were
verified by direct inspection of `cmd/forecastiq/migrate.go`, `internal/api/router.go`,
and the workflow/scripts (not just review-agent output).

## 2. Findings

### Critical

**DRB-WP23-001 (Critical)** — `forecastiq migrate --confirm` is not a valid CLI
command (`migrate.go` accepts `up|down|force|status`); the remote deploy step
(`ci.yml` deploy-api, `deploy/scripts/deploy.sh`) aborts on every run. Verified:
`ci.yml:229` vs `migrate.go` switch. Fix: `migrate up` (or implement `--confirm`).

**DRB-WP23-002 (Critical)** — rsync trailing slashes (`migrations/ deploy/`) flatten
the release layout; every subsequent `${RELEASE_DIR}/deploy/...` reference (systemd
unit cp, Caddyfile cp, smoke-test path, rollback smoke step) points at a nonexistent
path. In `deploy.sh` step 4 the failure is silently swallowed (heredoc lacks
`set -e`). Verified at `ci.yml:211-225`. Fix: drop trailing slashes + `set -euo
pipefail` in the remote heredoc.

**DRB-WP23-003 (Critical)** — smoke test requests `/rankings` and `/admin/health`
unversioned; the router mounts them under `/api/v1` → 404 on every run → every
deploy and rollback is flagged failed. Verified: `smoke-test.sh:42-45` vs router.

**DRB-WP23-004 (Critical)** — `actions/{upload,download}-artifact@v4` do not
preserve the executable bit; the binary lands 644 on the VPS → `migrate` exits 126
and systemd fails 203/EXEC. Fix: `chmod +x` after download.

### High

**DRB-WP23-005 (High)** — smoke-test `curl -f` + `-w "%{http_code}"` + `|| echo 000`
yields a corrupted two-line status string; the expect-401 check can never pass.
**DRB-WP23-006 (High)** — deploy privilege model unimplemented: `VPS_USER=deploy`
is never provisioned by `bootstrap.sh` (no user, no key, no sudoers); remote steps
need root; bootstrap re-runs `chown` releases away from the deployer.
**DRB-WP23-007 (High)** — placeholder domain `api.forecastiq.example` is
force-copied to `/etc/caddy/Caddyfile` on every deploy, clobbering operator fixes
and breaking ACME. Template the domain or commit the real one.
**DRB-WP23-008 (High)** — Makefile `deploy-release` writes an absolute local path
into `checksums.txt`; `sha256sum -c` on the VPS always fails (manual deploy path
has never worked).
**DRB-WP23-009 (High)** — `apt-get install caddy` fails on Ubuntu 22.04 (not in
jammy archives); bootstrap dies before firewall hardening is applied. Add the
Caddy APT repo (as done for grafana-agent).

### Medium

**DRB-WP23-010 (M)** — `StrictHostKeyChecking no` across ci.yml/deploy.sh/rollback.sh
→ MITM-able deploy channel; pin the host key.
**DRB-WP23-011 (M)** — `/var/log/caddy` root-owned; Caddy (user `caddy`) fails to
open its access log → config load failure.
**DRB-WP23-012 (M)** — Cloudflare Pages CNAME `"${var.domain}.pages.dev"` is not a
valid Pages hostname; introduce `var.pages_project`.
**DRB-WP23-013 (M)** — Terraform state backend commented out while `neon_role`
passwords live in plaintext local state; commit one backend before first apply.
**DRB-WP23-014 (M)** — `aquasecurity/trivy-action@master` unpinned in a blocking
gate; also trivy secret-scan (spec §1) not configured.
**DRB-WP23-015 (M)** — rollback target selection: substring `grep -v` filter +
mtime ordering + pipefail kills the script on empty releases dir before the guard.

### Low

**DRB-WP23-016 (L)** — `ufw --force reset` leaves a firewall-down window on re-run.
**DRB-WP23-017 (L)** — no root check in bootstrap despite requiring root.
**DRB-WP23-018 (L)** — Neon app role is the database owner (full DDL), contradicting
the DML-only comment and the separate migrate role.
**DRB-WP23-019 (L)** — bootstrap steps 5/6 reference a release layout that never
exists under either the buggy or the fixed rsync form; `|| echo` hides it.

## 3. Scope coverage vs docs/delivery/02-ci-cd.md

**Present**: PR jobs incl. new trivy CVE gate · build-release (binary + checksums,
90d retention) · deploy-api job with approval gate, symlink-flip release model,
readyz loop, last-5 retention · bootstrap script · Caddyfile + hardened systemd
unit · rollback script with timed <5 min drill · Terraform DNS + Neon DB/roles ·
Makefile deploy targets.

**Absent**: ❌ artifact signing (cosign keyless — WP-23 says "build, **sign**,
deploy") · ❌ Cloudflare Pages deploy job ("Pages setup") · ❌ scheduled jobs
(nightly property run, weekly gitleaks history, monthly restore/rollback drill,
spec §3) · ❌ migration dry-run against prod-schema copy · ❌ notify webhook.

## 4. Decision

**REJECTED.** Findings 001–009 must be fixed; the absent "sign" and "Pages" scope
items must be delivered or explicitly descoped by decision log; 010–015 fixed or
tracked. The rollback design (symlink flip + retention + timed drill) and the
hardening posture are sound — the package fails because the pipeline was never
executed end-to-end. Re-review requires: fixes + green six-job CI on the new SHA,
plus evidence of at least one full deploy + rollback rehearsal (scratch VPS or
documented dry-run per the WP-23 test spec).
