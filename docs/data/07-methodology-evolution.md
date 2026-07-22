# ForecastIQ — Methodology Evolution (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: `docs/domain/03-metric-methodology.md` §9; BR-INV-01..03; BR-RANK-07; PC-02

---

## 1. Versioned Artifacts

| Artifact | Version format | Storage | Change policy |
|----------|---------------|---------|---------------|
| Methodology (formulas, thresholds, rules) | `2026.1` (year.sequence) | Code + config in repository; served by `GET /rankings/methodology` | New version = new rows; old never rewritten |
| Weights | `w-2026.1` or `custom:<sha256-8>` | Default in config; custom echoed per request | Same |
| Condition taxonomy | `1` (integer) | Adapter mapping tables (versioned in code) | Additive codes = new version; no retroactive remap |
| Adapter | semver | Recorded per collection | Normalization semantics change = minor/major bump |
| Provider schema | Per-provider string | Recorded per collection | New schema = new version; old rows keep original |

## 2. Derived-Result Evolution Policy (binding)

**All derived results are versioned immutably — never updated in place.**

| Trigger | What happens | Rows |
|---------|--------------|------|
| Late observation | New match pairs; affected metrics/rankings recomputed | New rows; old rows get `superseded_by` |
| Corrected observation | Rematch to correction; recompute affected scope | Same as above |
| Matching-rule change | Requires methodology version bump; recompute on demand (admin) | New rows under new methodology_version |
| Methodology version change | Rankings recomputed on demand; metrics on next batch with new version | New rows; both versions queryable |
| Weights change (default) | New weights_version; recompute | New rows |
| Custom weights (API request) | Computed on demand, echoed in response, **not stored** (day metrics excepted) | No stored rows for custom weights in MVP (ranking recompute with custom weights is admin-only, stored with `custom:<hash>` version) |
| Adapter upgrade | Prospective only (new collections carry new adapter_version) | No retroactive rewrite; replay available for stored payloads (90 d window) |
| Condition mapping correction | Prospective only; historical remap via replay if justified | Documented procedure |
| Provider schedule change | Coverage baseline updated prospectively; prior periods use then-active schedule | Schedule versions stored in configuration audit trail |

## 3. Recalculation Mechanics

```text
RecomputeScope(scope = {provider?, location?, horizon?, period?}):
  1. Select affected matched_evaluations (immutable inputs)
  2. Recompute metrics with CURRENT methodology version
     → new accuracy_metrics rows
     → set superseded_by on replaced rows (same logical key, older calculated_at)
  3. Recompute rankings from new metrics
     → new provider_rankings rows + supersede links
  4. Audit event: rankings.recompute_triggered (scope, actor, methodology_version)
  5. Event: accuracy.calculated → operations (freshness refresh)
```

- Completion within 2 batch cycles (≤ 1 h, BR-INV-02).
- Byte-identical reproducibility: same inputs + same methodology_version → same outputs (property 11; verified by replaying test vectors against historical versions in CI).

## 4. Historical Verification vs. Publication

When methodology changes (e.g., 2026.1 → 2027.1):
1. **Verification pass:** recompute a reference period with the OLD version → must reproduce stored values byte-identically (proves input integrity).
2. **Publication pass:** recompute with NEW version → new rows published (latest per logical key serves the API).
3. UI/API exposes `methodology_version` on every derived payload; methodology page (`/rankings/methodology`) carries `change_history[]`.

## 5. User-Facing Identification

Users can always determine which methodology produced a displayed result:
- API: `methodology_version` + `weights_version` in every derived payload (API-08).
- UI: version badge in data views; S-06 Methodology page with formulas, weights, thresholds, change history.
- CSV exports: metadata header includes methodology + weights versions.
- Ties/statuses: computed per the version that produced the row (a row ranked under 2026.1 stays ranked under its recorded rules).

## 6. Methodology Registry Implementation

- **Configuration, not a database table** (reconciliation verdict, doc 04 §1): registry lives in Go code (`internal/analysis/methodology/registry.go`) — formula implementations, weight sets, thresholds, version constants.
- `/rankings/methodology` serves the active registry as JSON (formulas as plain-language + LaTeX-style expressions, not executable code).
- Change history: maintained in the methodology document + registry changelog constant.
- Revisit table storage only if per-workspace methodology selection lands (Level 3).

## 7. Formula Change Governance (R-07 mitigation)

1. Formula changes require: methodology document version bump + ADR-level review note + test vector updates + property-test updates.
2. All 5 test vectors (methodology §10) + 11 property invariants (§11) run in CI on every change.
3. Worked example (methodology §8) reproduced as an integration test — any divergence blocks merge.
4. Code review checklist item: "Does this change alter any stored value semantics? → version bump required."

## 8. Cross-Reference

- Methodology (normative): `docs/domain/03-metric-methodology.md`
- Invalidation rules: business rules BR-INV-01..03
- Evaluation workflow: `docs/workflows/04-evaluation-and-ranking.md`
- Backfill/reprocessing: `docs/workflows/06-backfill-and-reprocessing.md`
- ADR-010 (composite scoring decision)
