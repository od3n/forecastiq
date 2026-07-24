# ForecastIQ — WP-16 Forecast Evolution API: Delivery Review Board

**Review date**: 2026-07-24
**Work package**: WP-16 — Forecast Evolution API (Forecast-vs-Actual)
**Reviewed SHA**: `14c168b29271694f689ad01dd4d69a4b2378039e` (`14c168b`)
**Decision**: **ACCEPTED**

---

## 1. Verification of evidence

| Check | Result |
|-------|--------|
| Commit identity: local HEAD == `git ls-remote origin` == CI head | ✅ all `14c168b` |
| CI run **30060501063** (`pull_request`, head `14c168b`) | ✅ **success** |
| Six mandatory jobs green, none skipped/cancelled | ✅ `backend-checks`, `backend-integration`, `migrations`, `api-contract`, `security`, `image` |
| Dependency gate: WP-08 + WP-10 + WP-15 Accepted | ✅ (registry lines 8, 10, 15) |
| Docs-only lineage since code+test tip `aa4d80c` | ✅ (`14c168b` = report + registry) |

## 2. Scope review (6/6 approved items + acceptance)

Independently spot-checked (implementation report §3):

- **DR-02 issuance selection**: the `DISTINCT ON (provider_id, target_time) … ORDER BY … forecast_horizon_minutes DESC WHERE forecast_horizon_minutes ≤ requested` correctly yields the exact horizon when present and the nearest shorter otherwise; each point exposes its actual `issued_at` + `horizon_minutes`. Verified by the integration test (3 hourly points at horizon 1440) and the unit test.
- **Gaps as absences**: a forecast hour without an observation stays on the line but is absent from `observations[]` and excluded from day metrics (integration: 3 points / 2 observations / sample_count 2).
- **Day metrics**: MAE/RMSE/Bias via the WP-12 `eval.Continuous` kernel under provenance weights; MAE 1.0 / Bias 0 reproduced; `error_band_mae` = pooled MAE.
- **Date-in-tz**: `time.ParseInLocation` in the location zone → UTC `[from, to)`; 404 on unknown location; 422 on missing/invalid params (all covered).
- **Envelope**: observation freshness, provenance mix, attribution, per-provider `provider_unavailable` warnings; ETag present; response < 20 KB (size governance).

## 3. Architecture + security assessment

- FvA read lives in the analysis module (owns `eval`, already reads snapshots + observations); reuses the WP-15 `ReadService`/`analysispg` read repository. No new cross-module edge; correct dependency direction.
- Variable→column mapping is a validated closed switch; all query values parameterized (`provider_id = ANY($2)`, times, horizon). No injection surface. Reads are live-row/quality-scoped (`superseded_observation_id IS NULL`, `quality_flag <> 'suspect'`).
- No migration, no credentials, no external calls. Public per AUTH-08; caching public-class only.

## 4. Adversarial checks (no defect found)

- **Horizon boundary**: `≤ requested` includes the exact horizon; a request below every stored horizon yields no point for that hour (line break), never a longer-horizon substitution.
- **Multiple issuances per hour**: `DISTINCT ON` deterministically keeps one row (largest horizon ≤ requested) per (provider, target hour).
- **All-absent providers**: yields empty series + `observations_unavailable` freshness, not a partial result (§4.2 rule 6).
- **tz correctness**: a stored location always has a valid IANA zone (create-time validated); the handler falls back to UTC defensively rather than 500 on an impossible bad zone.
- **Determinism**: series + day-metric output ordered by the requested provider list.

## 5. Findings

| Finding | Severity | Summary | Disposition |
|---------|----------|---------|-------------|
| DRB-WP16-001 | Low (informational) | `/forecast-comparison` uses the 60 s analysis cache class; the contract's per-date max-age (past 300 s / today 60 s) is not yet differentiated. | Non-blocking; ETag/304 + LRU apply, only past-date `max-age` differs. Documented deviation; revisit with a per-request TTL hook. |

No Critical/High/Medium finding.

## 6. Decision

**ACCEPTED.** WP-16 delivers the S-05 Forecast-vs-Actual endpoint — DR-02 issuance selection, in-memory day metrics reusing the WP-12 kernel, observation gaps as absences, provenance + freshness, and date-in-location-tz interpretation — CI-verified green on the exact code+test SHA `14c168b` including the `api-contract` drift gate and the real-PG `backend-integration` job. The adversarial review found no defect; one Low informational note (DRB-WP16-001, cache max-age) is non-blocking.

**Accepted Implementation SHA `14c168b`.** PR #14 ready to merge to `main`. **WP-17 (Accuracy Analytics API) becomes eligible** — it depends on WP-13 + WP-15 (both Accepted) and completes trend bucketing (tz-aware `date_trunc`) + provider grid mode over the WP-15 `/accuracy` baseline.
