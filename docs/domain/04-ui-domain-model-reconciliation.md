# ForecastIQ — UI ↔ Domain Model Reconciliation

**Version**: 1.0 (UI ↔ Backend Reconciliation Board)
**Status**: Authoritative — amends `docs/domain/01-domain-model.md` where marked
**Principle**: Avoid creating entities merely to mirror UI components. Every proposed change below is justified by a reconciled UI requirement and evaluated against the approved architecture (modular monolith, PostgreSQL, no new stores).

---

## 1. Entity-by-Entity Verdict

| Entity (board checklist) | Verdict | Rationale |
|--------------------------|---------|-----------|
| User | **Required for MVP** — amend (add preference fields, §2.1) | S-07/S-09 require default location + display preferences |
| Workspace | **Required** — unchanged | Single system workspace (domain model §8.2); schema-ready for Level 3 |
| WorkspaceMembership | **Not required for MVP** | Single workspace; role on `users` suffices (AUTH-06). Level 3 RBAC introduces it. No schema reservation needed beyond `workspace_id` columns (already present). |
| Location | **Required** — unchanged | S-01/S-02/S-12 |
| Provider | **Required** — unchanged (public exposure amended, §3) | S-03/S-11 |
| ProviderConfiguration | **Required** — unchanged | S-11 |
| ForecastCollection | **Required** — unchanged | S-13, health, lineage |
| ForecastSnapshot | **Required** — unchanged | S-05, lineage; supports deferred Forecast Evolution without change (§4) |
| ObservationCollection | **Not required** | Observations collected directly into `observations` with provenance (OC-01..06); a separate collection entity would mirror UI that doesn't exist. Observation collector health derives from `observations` aggregates (§5). |
| Observation | **Required** — unchanged | Ground truth; S-01 context, S-05 |
| ForecastObservationMatch (MatchedEvaluation) | **Required** — unchanged | Lineage; recomputation |
| EvaluationResult (AccuracyMetric) | **Required** — unchanged | S-02/S-03/S-04 |
| AggregatedMetric | **Can be represented by existing entity** | AccuracyMetric rows with `aggregation` semantics via period_start/end; daily/weekly/monthly buckets computed at query time over stored metric rows — no separate entity |
| RankingResult (ProviderRanking) | **Required** — unchanged | S-01/S-02/S-03 |
| MethodologyVersion | **Required for MVP** — as configuration, not table | Methodology is code + config versioned in the repository; `/rankings/methodology` serves the registry. A DB table is unnecessary for MVP (single active version + change history in docs). Verdict: **derive from configuration**; revisit if per-workspace methodology selection lands (Level 3). |
| ExportJob | **Deferred, not reserved** | MVP exports are client-side (C-18); GDPR export needs a minimal job record — represent as a row in a small `export_jobs` table (§2.2) scoped to AUTH-09 only. Not a general report engine. |
| Alert / AlertRule | **Deferred (Level 3)** — `workspace_id` reservation already exists (domain model §8.2); no further preparation | NP-03; removed from MVP UI (C-05) |
| AuditEvent | **Required** — unchanged | S-14 |

## 2. Approved Schema Amendments

### 2.1 `users` — preference fields **[AMENDS domain model §3 ERD]**

```
USER {
  …existing fields…
  uuid default_location_id FK→locations NULL   -- S-07 onboarding, S-09 preferences
  jsonb  preferences NOT NULL DEFAULT '{}'      -- {tz_display: "location"|"browser", …}
}
```

- Justification: DR-04 (onboarding), BR-TZ-03 (timezone toggle), S-01 default-location resolution.
- `default_location_id` is nullable (public users have no preference; first active location is the fallback).
- FK to locations: SET NULL on location disable is **wrong** (locations are never deleted; status change only) — FK remains valid; validation ensures `status = active` at write time.
- Mutable table (per domain model §11 convention) — `updated_at` present.

### 2.2 `export_jobs` — GDPR export tracking **[NEW, minimal]**

```
EXPORT_JOB {
  uuid id PK
  uuid user_id FK (requested_by; admin-triggered: target user)
  uuid target_user_id FK
  string status            -- pending | completed | failed
  string object_key        -- file on payload volume (same store; 24h expiry)
  timestamp expires_at     -- download link validity (24h)
  timestamp completed_at
  string error_message
  timestamp created_at
}
```

- Justification: AUTH-09 async export with download link; one active job per user (409 guard).
- Not a general report engine: scope is account-data JSON only (user row + keys list + created-resources list per AUTH-09). Weather-data CSV exports are client-side (C-18).
- Retention: rows + files deleted after `expires_at` by the same retention job family.

### 2.3 Circuit breaker state — persistence decision **[AMENDS FC-09 implementation]**

FC-09 specifies circuit breaker behaviour but not storage. The reconciliation requires persistence because:
- `/admin/health` must report `circuit_state` and "half-open probe in {n}s" (S-10) — impossible from in-memory state after restart.
- A process restart must not reset an open circuit (safety: prevents hammering a failing provider).

Decision: **DB-backed circuit state** in a small table (not Redis — excluded from MVP):

```
PROVIDER_CIRCUIT {
  uuid provider_id PK FK
  string state                 -- closed | open | half_open
  integer consecutive_failures
  timestamp last_failure_at
  timestamp opened_at
  timestamp next_probe_at
  timestamp updated_at
}
```

- One row per provider (not per provider-location — FC-09's breaker is per provider).
- Updated transactionally with collection failure/success recording (no extra round trip in the hot path beyond one upsert).
- Satisfies multi-instance safety ahead of promotion (SKIP LOCKED scheduling already assumes possible second instance).

### 2.4 No other schema changes

The following UI needs are satisfied **without** schema changes (verified against indexes in domain model §11 and `docs/data/01-query-and-index-requirements.md`):

| UI need | Satisfaction |
|---------|--------------|
| Observation context line (S-01) | Query: latest observation per location — indexed `(location_id, observed_at)` |
| Collection window (S-02/S-03) | MIN/MAX over snapshots — indexed `(provider_id, location_id, target_time)` |
| Provider grid (S-03) | Scan over `provider_rankings (provider_id, …)` — existing composite index |
| FvA payload (S-05) | Snapshots by `(location_id, target_time, horizon)` + observations by `(location_id, observed_at)` — existing indexes |
| Day metrics (S-05) | Computed over ≤ 24×2 matched pairs at query time (cheap; no storage) |
| Health aggregates (S-10) | `forecast_collections (provider_id, location_id, requested_at DESC)` — existing index |
| Engine lag (S-10) | MAX(calculated_at) over accuracy_metrics — existing index |
| Adapter version public (S-03) | Latest successful collection per provider — existing index |
| Next scheduled run (S-10) | Derived: schedule interval + last slot claim (scheduler module state, `collection_schedules` table per architecture §1) |

## 3. Relationship and Rule Corrections

| # | Finding | Correction |
|---|---------|------------|
| D-01 | Screen inventory §4 mapped S-03 to "`/forecast-collections` (public health subset)" — implied public exposure of an admin endpoint | Corrected: collection window exposed via `/accuracy/summary` (C-08); `/forecast-collections` remains admin-only. No domain change. |
| D-02 | `providers.attribution_text/url` is mutable config — UI footer depends on it per BR-ATTR-01 | Confirmed correct; audit-logged on change (admin mutation). No change. |
| D-03 | Observation uniqueness `(source, location_id, observed_at)` supports "latest observation" query but not per-variable freshness | Acceptable: BR-FRESH observation thresholds apply per location-source, not per variable. No change. |
| D-04 | `users.role` is a string enum (`admin`/`user`) — S-14 UI offers no role editing | Correct for MVP (AUTH-06: role assignment at bootstrap/operation level). Role-change endpoint **not added** — documented as Level 3 RBAC. UI must not render role editing. |
| D-05 | Missing lifecycle state: export jobs | Added `export_jobs.status` enum (§2.2). |
| D-06 | Missing uniqueness: export jobs active-per-user | Partial unique index on `(target_user_id) WHERE status = 'pending'` → 409 guard. |
| D-07 | Provenance gap: `observation_context` in rankings payload carries no quality_flag | Added `quality_flag` to the context block (latest observation may be `corrected`; UI badge shows provenance, quality weighting note in tooltip). |
| D-08 | Ownership: `export_jobs` ownership-bearing? | Yes — carries `workspace_id` (NOT NULL, default system workspace) per domain model §8.2 convention for ownership-bearing mutable tables. |

## 4. Deferred-Capability Model Impact (verification)

| Deferred capability | Model impact now | Verification |
|---------------------|------------------|--------------|
| Forecast changes (DR-06) | None | `LAG(value) OVER (PARTITION BY provider_id, location_id, target_time ORDER BY issued_at)` — runs over existing snapshot rows + indexes |
| Consensus/disagreement (DR-07/10) | None | Aggregate over snapshots per target_time — existing rows |
| Confidence rating (DR-08) | None (methodology first) | Would add fields to ranking rows only when methodology defines it — additive |
| Alerts | Reservation exists | `workspace_id` on future tables; event seam names fixed (ADR-006) |
| Issuance-axis evolution (C-13) | None | Same snapshot access path as changes feed |
| Reports engine | None | `export_jobs` (§2.2) is GDPR-scoped only; a general engine is a future entity, not reserved |

## 5. Invariants Reaffirmed (no UI-driven erosion)

- Snapshot/observation immutability (BR-01, PC-01): no UI action mutates pipeline entities. Replay creates new collections (domain §4.8). Recompute creates new metric/ranking rows (BR-INV-01, BR-RANK-07).
- No cascade deletes anywhere the UI can trigger (location/provider disable = status only; user deletion removes only personal rows per AUTH-09 — weather data untouched).
- Null metrics remain null end-to-end (methodology §5): UI shows "—", API returns null, CSV exports empty cell. Zero-tolerance for NaN/0 substitution.
- Workspace denormalization tradeoff (domain §8.2) unaffected: no UI requirement justifies adding `workspace_id` to child pipeline tables.
