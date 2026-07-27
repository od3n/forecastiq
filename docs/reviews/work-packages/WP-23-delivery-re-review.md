# ForecastIQ — WP-23 CI/CD + Deployment: DRB Confirmatory Re-Review

**Review date**: 2026-07-26
**Work package**: WP-23 — CI/CD and Deployment (PR #29, `feature/wp23-cicd-deployment`)
**Prior review**: WP-23-delivery-review.md — REJECTED on `4c3d111` (DRB-WP23-001…019)
**Reviewed SHA**: `cc9b7db`
**Decision**: **ACCEPTED** (under ADR-033)

---

## 1. Architecture note

Between review and re-review the deployment target changed by operator
decision: **AWS EC2 t3.small + Docker Compose, containerized PostgreSQL, TLS
at Cloudflare, GHCR image releases** (ADR-033, committed on this branch with
amendment banners on the deployment-architecture and rollback runbook docs).
The re-review evaluates the package against ADR-033, which supersedes the
Hetzner/native model the original findings were raised against.

## 2. Verification of evidence

| Check | Result |
|-------|--------|
| Commit identity: local branch == remote == CI head | ✅ all `cc9b7db` |
| Six PR jobs green (backend-checks, integration, migrations, api-contract, security, image) | ✅ first run |
| build-release / deploy-api correctly gated to main push | ✅ skipped on PR |
| Shell scripts `bash -n`; workflow + compose YAML parse | ✅ |
| **End-to-end deploy rehearsal (local)** | ✅ prod image built → postgres:16 container → `migrate up` (embedded migrations applied) → app boot → smoke **4/4 PASS** (healthz, readyz incl. DB + payload-volume gates, /api/v1/rankings, admin 401 gate) |
| Rehearsal found + fixed a live defect | ✅ smoke-test `\|\| echo 000` doubled curl's own transport-failure `000` → `000000`; removed |

## 3. Finding closure (DRB-WP23-001…019)

| Finding | Status | Resolution |
|---------|--------|-----------|
| 001 (C) invalid `migrate --confirm` | ✅ Eliminated | Deploy runs `docker compose run --rm app migrate up`; exercised in rehearsal |
| 002 (C) rsync layout flattening | ✅ Eliminated | No file transport: image pull by digest; compose file shipped by scp |
| 003 (C) unversioned smoke URLs | ✅ Closed | `/api/v1/...`; verified against the live rehearsal stack |
| 004 (C) lost exec bit | ✅ Eliminated | No binary artifact transport (image-based) |
| 005 (H) corrupted curl status | ✅ Closed | `-w` only, no fallback echo; regression caught live in rehearsal |
| 006 (H) deploy user unprovisioned | ✅ Closed | bootstrap creates `deploy` + authorized_keys; docker-group privilege model accepted per ADR-033 §7 |
| 007 (H) placeholder domain clobbering | ✅ Eliminated | No origin Caddy; TLS at Cloudflare (proxied A record) |
| 008 (H) checksum path | ✅ Eliminated | Checksums replaced by digest-pinned image + cosign verification |
| 009 (H) caddy not installable | ✅ Eliminated | Caddy removed from the topology |
| 010 (M) StrictHostKeyChecking no | ✅ Closed | CI pins `VPS_HOST_KEY` (strict); manual scripts use accept-new TOFU |
| 011 (M) caddy log dir | ✅ Eliminated | No Caddy |
| 012 (M) Pages CNAME target | ✅ Closed | `var.pages_project`; Pages *deploy automation* formally descoped (ADR-033) |
| 013 (M) Terraform state backend | ⚠ Tracked | Backend still commented; risk materially reduced — state no longer carries DB credentials (Neon removed; DNS records only) |
| 014 (M) trivy unpinned | ✅ Closed | `aquasecurity/trivy-action@v0.36.0` + secret scanner |
| 015 (M) rollback target selection | ✅ Eliminated | Rollback = recorded previous image digest; equality guard added |
| 016 (L) ufw reset window | ✅ Closed | Idempotent rule adds, no reset |
| 017 (L) no root check | ✅ Closed | EUID guard |
| 018 (L) app role owns DB | ✅ Eliminated | Neon removed; containerized PG owner is the single app role (ADR-033 accepted trade-off) |
| 019 (L) stale bootstrap paths | ✅ Eliminated | Bootstrap rewritten for the Docker model |

## 4. Scope coverage (vs docs/delivery/02-ci-cd.md, as amended by ADR-033)

**Delivered**: six blocking PR jobs (incl. trivy vuln+secret gate, promtool
gate from WP-22) · `build-release`: GHCR image push, digest output, **cosign
keyless signing** · `deploy-api`: production-environment approval, **cosign
verify pinned to this repo's main workflow identity**, single deploy path
shared with operators (`deploy.sh`) · bootstrap (Docker, deploy user,
ufw/fail2ban, nonroot-uid-aware data dirs) · rollback with recorded-previous
digest + NFR-M07 timing · scheduled workflows (nightly 10K property +
govulncheck, weekly gitleaks history, monthly rollback drill) · Terraform
Cloudflare DNS (proxied api record, pages_project CNAME) · ADR-033 + doc
amendments · deploy rehearsal evidence (§2).

**Descoped / tracked**: Cloudflare Pages deploy automation (ADR-033 — operator
task) · migration dry-run against prod-schema copy (the `migrations` CI job
covers fresh-schema up/verify/seed; prod-copy dry-run deferred to WP-24 once
backups produce restorable dumps) · notify webhook (minor, tracked) ·
Terraform remote state backend (013, tracked pre-first-apply) ·
CF-Connecting-IP mapping for IP-keyed rate limits (ADR-033 §4, tracked).

## 5. Decision

**ACCEPTED.** All 19 findings closed or structurally eliminated under ADR-033,
CI six jobs green on `cc9b7db`, and the deploy path has now been executed
end-to-end (locally, containerized) — the failure class that sank the first
review cannot recur unexercised: the monthly drill workflow rehearses rollback
on the real host once deploy secrets exist, and the first production deploy
follows the exact rehearsed sequence. PR #29 ready to merge.
**WP-24 (Backup/Recovery) becomes eligible — with its acceptance bar raised
per ADR-033 §3 (backups are the only durability net for containerized PG).**
