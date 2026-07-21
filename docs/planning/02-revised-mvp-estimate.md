# ForecastIQ — Revised MVP Estimate

**Version**: 1.0 (Phase 0 Amendment)
**Status**: Authoritative
**Supersedes**: story-point estimates in `docs/phase-0-business-analysis/08-user-stories.md`
(144 uncalibrated points replaced by engineer-day ranges)

---

## 1. Estimating Basis

- Unit: **engineer-day** (focused 8-hour day). No story points: without calibrated
  velocity they were false precision (amendment mandate).
- Ranges reflect technical unknowns; the planning number is the **upper bound** for
  commitment, the lower bound for sequencing.
- Assumes Go + React experience; add ~25% if unfamiliar.

## 2. Epic Estimates (from `docs/requirements/03-user-stories.md`)

| Epic | Engineer-days (low–high) | Dependencies | Principal risks |
|------|--------------------------|--------------|-----------------|
| E1 Forecast Collection | 13–16 | Provider keys (D-01), schema design | Provider response quirks (R-01) |
| E2 Observation Collection | 4–5 | E1 schema | Provenance exposure of source API (R-03) |
| E3 Accuracy Analysis | 17–21 | E1, E2 | Statistical edge cases (R-07) |
| E4 REST API | 13–15 | E1–E3 read models | — |
| E5 Authentication | 5–6 | Supabase project (D-03) | Vendor integration friction (R-14) |
| E6 Dashboard | 16–20 | E4, E5 | State completeness discipline |
| E7 Administration | 9–11 | E4 | — |
| E8 Platform & Ops | 13–16 | Infra account (D-04) | Deploy pipeline first-time setup |
| **Total** | **90–110** | | |

Plus cross-cutting buffers:

| Item | Days |
|------|------|
| Phase 1 architecture (ADRs, C4, detailed design) | 5–7 |
| Observation quality spike (D-06) + ToS review support (D-05) | 3–4 |
| Stabilization, security pass, launch checklist | 5–7 |
| **Grand total (Level 2)** | **103–128 engineer-days** |

## 3. Calendar Plans

### One engineer

- 103–128 working days ≈ **5–6.5 calendar months** at sustainable pace
  (21 working days/month, minus ~15% for ops/interruptions → effective ~18 d/month
  → 5.7–7.1 months; plan **6.5 months**).
- Sequence: E8-foundation → E1 → E2 → E3 → E4 → E5 → E7 → E6 → stabilization.
  (Level 1 demo falls out naturally at the end of E3, ~week 6–7.)
- Milestones: M1 pipeline live locally (wk 7); M2 API complete (wk 14); M3 dashboard
  complete (wk 21); M4 launch-ready (wk 26).

### Two engineers (parallelized)

- Split: Eng-A backend pipeline + analysis (E1, E2, E3, E7); Eng-B API + auth +
  dashboard + ops (E4, E5, E6, E8), with E8 foundation done jointly in week 1.
- Coordination overhead ~10%; effective ≈ 1.8× → **3.5–4 calendar months**.
- Interface contract (OpenAPI stubs + schema) frozen end of week 2 to decouple.

## 4. Comparison to Phase 0 Plan

Phase 0 implied 144 story points across 8 epics with Kubernetes/Temporal/NATS — an
unbounded calendar commitment for 1–2 engineers (ARB Risk R-09). The revised plan is
**~30–40% smaller in feature surface** and **dramatically smaller in operational
surface**, with an explicit calendar bound.

## 5. Estimate Governance

- Actuals tracked per epic weekly; > 20% deviation triggers re-forecast and scope
  review (cut scope, never quality gates).
- Any scope addition requires: amendment note + estimate delta + what is removed or
  delayed in exchange (zero-sum discipline).
- Promotion-criteria work (Redis/NATS/etc.) is explicitly **not** in this estimate;
  each would be estimated at trigger time.
