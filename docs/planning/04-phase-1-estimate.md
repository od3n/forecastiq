# ForecastIQ — Phase 1 Estimate (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: `docs/planning/02-revised-mvp-estimate.md` (authoritative epic estimates — this document refines into work-package allocation without changing the committed total)

---

## 1. Basis

The Phase 0 Amendment estimate (103–128 engineer-days for Level 2) remains the commitment. Phase 1 architecture did not change scope; it converted epics into 27 work packages. This document maps packages to the epic budget and adds Phase-1-specific findings.

## 2. Work Package → Epic Mapping

| Epic (estimate) | Budget (d) | Packages | Allocated (d) |
|-----------------|-----------|----------|---------------|
| E1 Forecast Collection | 13–16 | WP-05 (part), WP-06, WP-07, WP-08 | 14–17 |
| E2 Observation Collection | 4–5 | WP-09, WP-10 | 4–5 |
| E3 Accuracy Analysis | 17–21 | WP-11, WP-12, WP-13, WP-14 | 17–21 |
| E4 REST API | 13–15 | WP-15, WP-16, WP-17, WP-18 | 13–15 |
| E5 Authentication | 5–6 | WP-03, WP-19 | 5–6 |
| E6 Dashboard | 16–20 | WP-20, WP-21 | 16–20 |
| E7 Administration | 9–11 | (admin endpoints within WP-15/18; admin screens within WP-21) | 9–11 |
| E8 Platform & Ops | 13–16 | WP-01, WP-02, WP-22, WP-23, WP-24 | 13–16 |
| Cross-cutting | 13–18 | WP-04, WP-25, WP-26, WP-27 + spikes | 13–18 |
| **Total** | **103–128** | 27 packages | **104–129** |

Delta vs. Phase 0 estimate: **+1 day** (reconciliation additions: circuit persistence, three new endpoints, export_jobs) — within estimating noise; total commitment unchanged (absorbed by WP-15/18 scope clarity).

## 3. Phase 1 Architecture Effort (this package)

Budgeted at 5–7 days (estimate cross-cutting line "Phase 1 architecture"): actual = this document set (~51 documents + 20 ADRs). Considered complete with the architecture report; no further architecture-phase work except amendment-triggered updates.

## 4. Calendar Confirmation

- One engineer: **6.5 months** plan (estimate §3) — sequence per `docs/delivery/05-implementation-sequence.md` §3.
- Two engineers: **3.5–4 months** with the documented split + week-2 interface freeze.
- Milestones M1 (wk 7), M2 (wk 14), M3 (wk 21), M4 (wk 26) unchanged.

## 5. Estimate Governance (carried forward)

- Actuals per package weekly; > 20% epic deviation → re-forecast + scope review (cut scope, never quality gates).
- Scope additions: amendment + estimate delta + zero-sum trade (what is removed/delayed).
- Promotion work (Redis/NATS/etc.) explicitly excluded; estimated at trigger time.
- Architecture changes during implementation: material deviations require ADR + this document update.

## 6. Estimating Assumptions (Phase 1 additions)

| Assumption | Impact if wrong |
|------------|-----------------|
| Go + React experience (estimate basis) | +25% if unfamiliar (estimate §1) |
| Open-Meteo API shape stable during build | Adapter rework ~1–2 d per drift event (R-01) |
| Supabase Auth integration friction low (E5 = 5–6 d) | Vendor friction → +2–3 d (R-14); JWKS path isolated for swap |
| No staging environment needed | If needed: +2 d setup + ongoing (scaling doc §1.9 trigger) |
| Single operator deploys | Second operator adds ~0 (pipeline already self-service) |

## 7. Cross-Reference

- Work packages: `docs/planning/05-implementation-work-packages.md`
- Sequence: `docs/delivery/05-implementation-sequence.md`
- Original estimate (authoritative): `docs/planning/02-revised-mvp-estimate.md`
