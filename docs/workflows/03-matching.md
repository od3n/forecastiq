# ForecastIQ — Matching Workflow (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: BR-MATCH-01..06; methodology §3; CE-01/CE-10/CE-11

---

## 1. Matching Dimensions

Every match is defined by: **provider × location × target time (UTC hour) × weather variable eligibility × observation source selection**. Matches are stored per snapshot–observation pair (variable eligibility is evaluated at metric time, not match time — a pair exists if the time-location match succeeds; per-variable nulls filter later).

## 2. Deterministic Algorithm (pseudocode, normative)

```text
MatchBatch():
  window = [now() − 30d, now() − 2h]          -- publication margin
  snapshots = SELECT unmatched snapshots in window (QX-01), chunked 5000

  FOR each snapshot S:
    hour = floor(S.target_time, 1h)            -- BR-MATCH-01: exact-hour, UTC only

    candidates = SELECT * FROM observations
      WHERE location_id = S.location_id
        AND observed_at = hour                 -- hourly source (MVP default)
        AND quality_flag != 'suspect'          -- BR-MATCH-05
      -- sub-hourly sources (future): observed_at BETWEEN hour−15m AND hour+15m

    IF candidates empty:
      CONTINUE                                  -- remains unmatched (coverage counts)

    chosen = ORDER candidates BY
      1. quality_flag = 'corrected' DESC        -- BR-MATCH-04: corrections preferred
      2. provenance_rank(observation_type) ASC  -- station(1) > interpolated(2)
                                                -- = reanalysis(2) > provider_estimated(3)
      3. ABS(observed_at − hour) ASC            -- nearest top of hour
      4. id ASC                                 -- final deterministic tiebreak
    LIMIT 1

    INSERT INTO matched_evaluations
      (forecast_snapshot_id, observation_id, provider_id, location_id,
       forecast_horizon_minutes, target_time, match_rule, time_delta_minutes)
    VALUES (S.id, chosen.id, ..., 'exact_hour', minutes(chosen.observed_at − S.target_time))
    ON CONFLICT (forecast_snapshot_id, observation_id) DO NOTHING
```

## 3. Rule Specification

| Rule | Implementation |
|------|----------------|
| Time basis | UTC only; no local-time/DST logic anywhere in the engine (BR-MATCH-06) |
| Exact-hour (MVP) | `floor(target_time, 1h) = observed_at`; both stored on the hour for hourly sources → equality |
| Sub-hourly tolerance | ±15 min ONLY for sources reporting < 1 h intervals (none in MVP; rule implemented behind source-capability flag); precipitation summed to hour; instantaneous variables take nearest-to-top reading (BR-MATCH-02) |
| Precipitation aggregation | 1-hour accumulated amounts on both sides (providers already report hourly accumulation) |
| Multiple observations | Provenance rank → corrected preference → nearest-to-hour → id (total order, no ambiguity) |
| Corrected observations | Preferred over `valid` for same source/time; existing matches to superseded observations trigger rematch (§5) |
| Incomplete observations | Per-variable eligibility at metric time (methodology §3 rule 3) — match exists, variable skipped if null |
| Suspect observations | Never matched (excluded from candidates; BR-MATCH-05) |
| Provider-specific policies | Adapter-declared match offset (e.g., :30 issuers); MVP default exact-hour for all; config point exists, unused |

## 4. Idempotency and Uniqueness

| Constraint | Effect |
|------------|--------|
| `UNIQUE (forecast_snapshot_id, observation_id)` | Same pair never stored twice; batch re-runs are no-ops |
| Batch selection = NOT EXISTS(match) | Already-matched snapshots never re-processed |
| Deterministic candidate order | Same inputs → same chosen observation (property-tested: permutation invariance over candidate arrival order) |
| No in-place match updates | A match changes only by **adding** a new pair row (rematch), never editing |

## 5. When a Match Changes After Creation

| Trigger | Behaviour |
|---------|-----------|
| Corrected observation arrives | New observation row (corrected) + old superseded. Rematch: for each MatchedEvaluation referencing the superseded observation → create new pair to the corrected observation (old pair retained for lineage). Affected metrics/rankings recomputed (BR-INV-01). |
| Late observation for previously unmatched snapshot | Next batch creates the pair naturally (snapshot still unmatched). Affected period metrics recomputed in next aggregation. |
| Methodology matching-rule change | Version bump; admin-triggered recompute over affected window; new pairs under new rule coexist with old (match_rule column distinguishes). |
| Snapshot replay (adapter fix) | New snapshots (new ids) → matched independently; original pairs untouched. |

A match NEVER changes due to: ranking recomputation, provider config changes, or UI actions.

## 6. Unmatched States

| State | Meaning | Query pattern |
|-------|---------|---------------|
| Unmatched (no observation yet) | target_time within publication margin OR source gap | NOT EXISTS — retried every batch until 30 d window passes |
| Unmatched (window expired) | No observation arrived within 30 d | Permanently unmatched; counts against nothing (observation-side gap, not provider coverage) |
| Ineligible pair | Match exists but variable null on one side | Excluded per-variable at metric time |

## 7. Batch Execution

- **Frequency:** every 30 min (scheduler job `analysis_batch`), after collection/observation jobs in the same cycle.
- **Chunking:** 5,000 snapshots per tx; total batch < 10 min at design volume (NFR-P06 headroom).
- **Event consumption:** `forecast.collected` / `observation.collected` mark eligibility immediately, but matching runs on schedule (events update backlog gauges, not immediate processing — avoids per-collection tx coupling).
- **Failure:** batch tx failure → rollback chunk, retry next cycle (idempotent). Per-chunk isolation prevents one bad row from blocking the batch (CE-11).

## 8. Observability

- `matching_backlog` gauge (unmatched eligible snapshots) — alert if growing over 3 cycles.
- `batch_duration_seconds{matching}` histogram.
- Log: `matching.batch_completed` with pairs_created, snapshots_scanned.

## 9. Cross-Reference

- Evaluation consumption: `docs/workflows/04-evaluation-and-ranking.md`
- Correction recomputation: methodology §9; BR-INV-01..03
- Query plan: QX-01 (`docs/data/04-index-and-query-plan.md`)
