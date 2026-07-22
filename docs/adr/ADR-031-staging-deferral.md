# ADR-031: Staging Environment Deferral

**Status**: Accepted (Phase 1)
**Date**: 2026-07-22

## Context
The prompt asks which environments are justified. The deployment architecture proposed local + CI + production with no standing staging. This ADR makes that decision explicit.

## Options considered
1. Standing staging environment (second VPS + second DB tier) — ~+$35–50/mo and ongoing maintenance for a single-operator team; value unproven at MVP.
2. **Three environments: local (docker-compose), CI (ephemeral), production. Production-adjacent validation via: migration dry-runs against prod-schema copies, Cloudflare Pages PR previews for the dashboard, smoke tests post-deploy, and < 5 min rollback.**
3. Staging-as-a-branch (long-lived preview deployment of main-next) — partial value, still needs infra; rejected.

## Decision
Option 2, per `docs/delivery/03-environments.md`.

## Rationale
- The risks staging typically catches are covered more cheaply: schema risk → dry-run against real schema copy; UI risk → Pages previews per PR; runtime risk → smoke tests + instant rollback; data risk → PITR.
- A single operator cannot meaningfully exercise a staging environment continuously — it would sit idle 99% of the time while still needing maintenance.
- Honest tradeoff: the < 30 s deploy gap + fast rollback substitutes for pre-production soak at this scale.

## Consequences
- (+) ~$40/mo and one whole environment's maintenance saved.
- (+) Every merge is production-validated by design (trunk-based discipline).
- (−) No place for destructive rehearsal except scratch instances spun up per-drill (monthly restore/rebuild drills cover this).
- (−) Real-provider integration testing is manual/weekly (rate limits) — fixtures cover the regression surface.

## Risks
A bad deploy reaching users — bounded by smoke tests + rollback SLO (< 5 min) + error-budget policy.

## Migration trigger
Second engineer onboarded, OR customer-facing SLAs, OR migration complexity exceeding dry-run confidence (e.g., multi-step data migrations) → standing staging.

## Review date
At second-engineer onboarding or Level 3 gate.
