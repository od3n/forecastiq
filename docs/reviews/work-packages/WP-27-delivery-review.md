# ForecastIQ — WP-27 Documentation and Demo Preparation: Delivery Review Board

**Review date**: 2026-07-27
**Work package**: WP-27 — Documentation and Demo Preparation (PR #33, `feature/wp27-docs-demo-prep`)
**Reviewed SHA**: `4b1f355` (post ADR-033 merge-up)
**Decision**: **REJECTED — the documentation half of the WP is absent; scripts are sound**

---

## 1. Context

WP-27 scope: *README refresh; **runbooks verified against reality**; methodology
page content review; demo script; attribution verification (BR-ATTR-01 every
surface); launch checklist*. The PR delivers two well-built scripts
(`demo.sh`, `launch-checklist.sh`) carrying prior-DRB remediations (correct
`status_of` capture, metrics separate-listener handling, OpenAPI python-quoting
fix). But the **documentation deliverables are missing**, and critically the
runbook rewrite that ADR-033 *explicitly deferred to "the WP-27 docs pass"* was
not done — so the runbooks now actively describe a topology that no longer
exists.

## 2. Findings

### High

**DRB-WP27-001 (H)** — Runbooks/architecture docs still describe the pre-ADR-033
topology under the ADR-033 banner; the deferred rewrite was not performed.
Verified:
- `docs/architecture/06-deployment-architecture.md` body: "Primary: **Hetzner
  Cloud VPS** + managed PostgreSQL (Neon…)", "Go binary support: Native
  (**systemd**)", "VPS + … + **Caddy**" — under a banner that says production is
  EC2+Docker.
- `docs/operations/05-deployment-and-rollback.md` body: steps still say
  `systemctl restart forecastiq` (no systemd unit under ADR-033).

ADR-033's own consequences section states these docs were "amended by reference
(banner notes added; **full rewrite deferred to the WP-27 docs pass**)". WP-27
is that pass and did not do it, so the scope item "runbooks verified against
reality" is unmet — an operator following the rollback runbook runs a command
that does not exist. Fix: rewrite the two doc bodies for the EC2/Docker/GHCR/
Cloudflare topology (deploy.sh image pull, `docker compose up -d`, rollback via
`FIQ_IMAGE` swap), consistent with the ADR and the WP-23/24/25 deliverables.

### Medium

**DRB-WP27-002 (M)** — `launch-checklist.sh` verifies runbooks/architecture
docs **exist**, not that they are accurate (§6 checks `[ -f … ]`). Combined with
001, the checklist would green-light a launch while the deployment runbook lies
about the topology. It should assert the docs are ADR-033-consistent (e.g. grep
that the rollback runbook references `docker compose`/`FIQ_IMAGE`, not
`systemctl`), turning "verified against reality" into an actual gate.

**DRB-WP27-003 (M)** — Attribution verification is partial vs "BR-ATTR-01 every
surface". `demo.sh` §8 checks attribution in one `/rankings` response;
`launch-checklist.sh` doesn't check attribution at all. The scope wants coverage
across every surface (rankings, accuracy, trends, FvA, dashboard footer). Add an
attribution gate to the checklist iterating the public read endpoints.

### Low

**DRB-WP27-004 (L)** — `demo.sh:99` fallback string "headers present via Caddy
in production" is stale: ADR-033 removed Caddy and WP-25 makes the app the
header-setting layer, so the headers are present in *all* environments. Update
the message.

## 3. Verified correct

Both scripts: `set -euo pipefail`; `status_of` avoids the `curl -f` "401000"
concatenation bug (prior DRB fix present); metrics URL is a separate listener
(not `BASE_URL:9090`); OpenAPI check uses correct python quoting; `/api/v1/...`
paths all exist in `router.go` (incl. `/api/v1/openapi.json`); manual gates
(D-05, D-06, perf baseline) honestly marked `skip`. README carries no stale
Hetzner/Caddy/systemd/Neon references.

## 4. Scope coverage

Demo script ✅ · launch checklist ✅ (but existence-only doc checks, 002) ·
runbooks verified against reality ❌ (001) · README refresh ✅ (clean) ·
attribution every surface ⚠ partial (003) · methodology page content review —
no artifact (S-06 single-sources the registry, so low-risk; note in re-review).

## 5. Decision

**REJECTED.** The demo and checklist scripts are acceptable; the WP fails
because its documentation half — specifically the runbook/architecture rewrite
ADR-033 handed to WP-27 — is absent, leaving launch runbooks that describe a
dead topology. Fix 001 (rewrite the two doc bodies for EC2/Docker), 002 (make
the checklist assert doc accuracy, not mere existence), 003 (attribution gate
across surfaces), 004 (demo string). Re-review requires green CI on the new SHA.
