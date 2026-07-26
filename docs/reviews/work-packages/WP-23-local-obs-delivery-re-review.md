# ForecastIQ — WP-23 Local Observability + S-16/S-17: DRB Confirmatory Re-Review

**Review date**: 2026-07-26
**Work package**: WP-23 (local slice) — PR #34, `feature/wp23-local-observability`
**Prior review**: WP-23-local-obs-delivery-review.md — REJECTED on `e7c2cb8` (DRB-WP23L-001…007)
**Reviewed SHA**: `9ceb37d`
**Decision**: **ACCEPTED**

---

## 1. Verification of evidence

| Check | Result |
|-------|--------|
| Commit identity: local branch == remote == CI head | ✅ all `9ceb37d` |
| CI runs **30214384604 + 30214384602** (seven jobs incl. frontend-checks) | ✅ success (first run) |
| Local: eslint, tsc, vitest 42/42 | ✅ |
| Production-bundle token canary: build with `NEXT_PUBLIC_DEV_TOKEN=<canary>` then grep bundle | ✅ canary absent — gate verified empirically |

## 2. Finding closure

| Finding | Status | Fix |
|---------|--------|-----|
| 001 (C) `obs-reset` destroys pgdata | ✅ Closed | `rm -sf prometheus loki promtail grafana` + explicit `docker volume rm` of the three obs volumes; `down`/`down -v` removed from obs targets |
| 002 (H) Promtail `msg` stream label | ✅ Closed | Label dropped; in-file comment cross-references DRB-WP22-009 |
| 003 (H) Compare clamp clobbers stored horizon | ✅ Closed | `setParams` gains `persist` option; the S-05 clamp passes `persist: false` |
| 004 (H) dev token unguarded | ✅ Closed | Injection gated to `NODE_ENV === "development"`; three duplicated `authHeaders()` collapsed onto exported `devAuthHeaders()`; canary-verified out of production bundles |
| 005 (M) horizon NaN guard | ✅ Closed | `Number.isFinite(n) && n > 0` before use |
| 006 (M) S-16 epoch date / wrong label | ✅ Closed | `completed_at: string \| null`, "not completed" fallback, label switches on picker state |
| 007 (L) prometheus comment/target | ✅ Closed | Comment corrected + secondary `host.docker.internal:9090` target for native runs |
| Info items (Grafana port 3000, engine-metrics panel gap, unlabeled timestamps) | ⚠ Tracked | Non-blocking; noted as follow-on polish |

## 3. Decision

**ACCEPTED.** All blocking findings closed on `9ceb37d`, seven jobs green
(runs 30214384604/30214384602). PR #34 is ready to merge to `main`. PR #35
(WP-19, stacked on this branch) must merge this branch and re-verify before its
own DRB.
