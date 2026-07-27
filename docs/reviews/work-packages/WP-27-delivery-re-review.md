# ForecastIQ — WP-27 Documentation and Demo Preparation: DRB Confirmatory Re-Review

**Review date**: 2026-07-27
**Work package**: WP-27 — Documentation and Demo Preparation (PR #33, `feature/wp27-docs-demo-prep`)
**Prior review**: WP-27-delivery-review.md — REJECTED on `4b1f355` (DRB-WP27-001…004)
**Reviewed SHA**: `a683231`
**Decision**: **ACCEPTED**

---

## 1. Verification of evidence

| Check | Result |
|-------|--------|
| Commit identity: local branch == remote == CI head | ✅ all `a683231` |
| Six PR jobs green | ✅ first run |
| `bash -n` on both scripts | ✅ |
| **Launch checklist run against a live compose stack** | ✅ 22 passed / 0 failed / 6 skipped (manual gates + attribution-on-empty-data) |
| Doc-accuracy gate exercised | ✅ "runbook references docker compose / FIQ_IMAGE" PASS; "no stale systemctl step" PASS |

## 2. Finding closure

| Finding | Status | Resolution |
|---------|--------|-----------|
| 001 (H) runbooks describe dead topology | ✅ | Rewrote `docs/architecture/06-deployment-architecture.md` §1–§10 (EC2 t3.small + Docker Compose, containerized PG, GHCR cosign image, Cloudflare proxied TLS, image-swap rollback, self-hosted-DB cost model) and `docs/operations/05-deployment-and-rollback.md` §1/§2/§4/§5 (deploy.sh pull→compose→migrate→readyz→smoke; rollback via `FIQ_IMAGE`). `grep` confirms no `systemctl`/`Hetzner`/`Neon`/`Caddy`/`rsync` left in the bodies |
| 002 (M) checklist existence-only doc checks | ✅ | `launch-checklist.sh` now asserts the runbook **references** `docker compose`/`FIQ_IMAGE` and **has no** `systemctl` — a real accuracy gate, verified live |
| 003 (M) attribution partial | ✅ | New checklist section iterates public read endpoints (rankings, accuracy/summary) asserting envelope attribution (skip on empty data pre-launch, must pass populated) |
| 004 (L) demo Caddy string | ✅ | Fallback message now "headers set by the app in all environments; WP-25 SecurityHeaders" |

## 3. Verified correct (carried forward)

Both scripts retain their prior-DRB fixes (`status_of` capture, metrics
separate-listener, OpenAPI python-quoting); README carries no stale topology
references; manual gates honestly `skip`. Methodology page content is
single-sourced from the engine via S-06 (`/rankings/methodology`), so "content
review" is structurally satisfied (no separate prose artifact to drift).

## 4. Decision

**ACCEPTED.** The documentation half now matches reality — the runbook/
architecture rewrite ADR-033 deferred to WP-27 is done, and the launch
checklist *enforces* that accuracy rather than merely checking file existence.
Six jobs green on `a683231`; checklist rehearsed end-to-end against a live
stack. PR #33 ready to merge. This closes the WP-22…WP-27 DRB queue except
**PR #35 (WP-19 web session auth)** and the tracked follow-ons (WP-26b; ADR-033
§4 security-group / CF-Connecting-IP).
