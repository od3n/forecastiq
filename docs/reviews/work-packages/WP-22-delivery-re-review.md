# ForecastIQ — WP-22 Observability: DRB Confirmatory Re-Review

**Review date**: 2026-07-26
**Work package**: WP-22 — Observability
**Prior review**: WP-22-delivery-review.md — REJECTED on `4a1e55b` (DRB-WP22-001…014)
**Reviewed SHA**: `693dabe` (`feature/wp22-observability`)
**Decision**: **ACCEPTED**

---

## 1. Verification of evidence

| Check | Result |
|-------|--------|
| Commit identity: local branch == remote == CI head | ✅ all `693dabe` |
| CI run **30211279024** (six jobs) | ✅ success (first run) |
| Six jobs green, none skipped/cancelled | ✅ backend-checks, backend-integration, migrations, api-contract, security, image |
| New mechanical gate: `promtool check rules` in backend-checks | ✅ 17 alert + 6 recording rules validated |
| Local: `go test -race ./...` | ✅ all packages |

## 2. Finding closure

| Finding | Status | Fix |
|---------|--------|-----|
| 001 (C) burn-rate precedence | ✅ Closed | `(1 − ratio) / (1 − 0.995)` parenthesized; healthy system ≈ 0.2 (verified by expression evaluation) |
| 002 (C) A5 permanently firing | ✅ Closed | `sum by (provider)(increase(...{status=~"success\|partial\|deduplicated"}[3h])) == 0` |
| 003 (C) A2 dead | ✅ Closed | Alert derives 5xx ratio from `http_requests_total{status_class="5xx"}` aggregated; `http_errors_total` now incremented in the RED middleware on ≥500 |
| 004 (H) A7 frozen gauge | ✅ Closed | `engine_lag_seconds` computed at scrape time from `max(calculated_at)` by the new `adapters/promexport.EngineCollector`; dispatcher's misleading `Set(0)` removed |
| 005 (H) A6/A14 label mismatch | ✅ Closed | Both divisions aggregated `sum by (provider)` on both sides |
| 006 (H) A10/A11 absent metrics | ✅ Closed | `promexport.BackupCollector` exports `forecastiq_backup_*` / `forecastiq_restore_test_*` from the WP-24 status file; metrics absent (alerts silent) until the file exists — unit-tested including absent-file and failed-status cases |
| 007 (H) A15 semantics | ✅ Closed | `changes(process_start_time_seconds[1h]) > 2` |
| 008 (M) unpopulated gauges | ✅ Closed | `evaluation_backlog` and `ranking_freshness_age_seconds` now DB-derived in EngineCollector (matched pairs newer than last aggregation; age of newest live ranking per location/horizon) |
| 009 (M) Loki msg label | ✅ Closed | `msg` no longer promoted to a stream label; queried via LogQL JSON filter |
| 010 (M) sanitizer bypass | ✅ Closed | Key matching normalizes case and strips `_`/`-`; patterns extended (passwd/jwt/bearer); camelCase variants covered by new tests |
| 011 (M) SLI deviation | ✅ Closed | `deduplicated` counted as success in `collection_success` recording rule (documented rationale in-file) |
| 012 (L) integration scrape test | ⚠ Tracked | Presence test remains unit-level; catalog metrics moved to promexport are covered by promexport unit tests. Follow-on item |
| 013 (L) payload GaugeFunc 0/0 | ⚠ Tracked | Unchanged; A9 remains guarded only for the healthy path. Follow-on item |
| 014 (L) dashboard drift | ⚠ Tracked | Panels unchanged; metric names referenced remain valid. Follow-on item |

## 3. Regression guard

`promtool check rules` now runs in the `backend-checks` CI job against both rule
files, so syntactically invalid or unparsable PromQL cannot land again. Semantic
correctness (thresholds, label matching) remains a review concern — this re-review
verified each rewritten expression by inspection against the registered metric
label sets in `internal/platform/metrics/metrics.go`.

## 4. Decision

**ACCEPTED.** All seven blocking findings and all four Medium findings are closed
on `693dabe` with six-job CI green (run 30211279024). Low findings 012–014 are
tracked as follow-on polish. PR #28 is ready to merge to `main`.
**WP-23 acceptance becomes eligible** per the WP-22 → WP-23 sequence.
