# ForecastIQ — Risk Register (Revised)

**Version**: 2.0 (Phase 0 Amendment)
**Status**: Authoritative — single source of truth for risks
**Supersedes**: risk sections in Phase 0 BRD §9, phase summary, and ARB §19 (which is
incorporated and re-assessed below)

Severity = Probability × Impact. Review cadence: monthly, or on any trigger event.

---

## 1. Critical and High Risks

| ID | Risk | P | I | Severity | Mitigation | Owner | Status |
|----|------|---|---|----------|-----------|-------|--------|
| R-01 | Provider API breaking change / schema drift | Med | High | **High** | Adapter pattern with versioned schema validation; schema_drift alerting (FC-11); contract tests against recorded fixtures; raw payload replay for recovery | Eng | Active |
| R-02 | Provider ToS violation (storage/redistribution) | Low | Critical | **High** | ToS review gate before public launch (D-05); attribution everywhere (BR-ATTR-01); store normalized data + derived metrics, not raw redistribution; OpenWeather swap path to Tomorrow.io | Operator | **Gate: blocks public launch** |
| R-03 | Observation quality inadequate for tropical maritime climate (Open-Meteo reanalysis vs. reality) | Med | High | **High** | Pre-launch quality spike vs. METMalaysia bulletins/literature (D-06); provenance typing + quality weighting; prominent UI caveats; NOAA path for US expansion | Eng | **Gate: blocks ranking publication** |
| R-04 | MVP delivery failure from scope creep | Med | Critical | **Critical** | Scope frozen at Level 2 definition; every addition requires amendment + estimate; promotion criteria prevent infra gold-plating; weekly scope check | Product/Eng | Active (primary residual risk) |
| R-05 | Single-engineer bus factor | High | High | **Critical** | Simple architecture (single binary); complete docs/ADRs/runbooks; IaC + pipeline automation; second-engineer onboarding target < 2 days | Eng | Active |
| R-06 | Operational complexity exceeds team | High→Low | High | **Med (residual)** | K8s/Temporal/NATS/Redis deferred with triggers; managed DB; Caddy auto-TLS; runbooks; this risk drove the architecture constraints doc | DevOps/Eng | Mitigated by design |
| R-07 | Incorrect metrics shipped (formula/edge-case bugs) | Low | Critical | **High** | Canonical methodology doc; test vectors as acceptance criteria; property-based testing (11 invariants); code review checklist for any formula change | Eng | Mitigated, continuously |
| R-08 | Methodology questioned externally | Low | High | **Med** | Full publication of methodology, weights, versions, sample sizes; reproducibility guarantee (PC-02); worked examples | Product/Eng | Mitigated by design |
| R-09 | Rate limits constrain collection (OpenWeather free tier) | Med | Med | **Med** | Hourly cadence (24/day/location); token bucket; staggered offsets; ≤ 10 MVP locations leaves 4× headroom; upgrade or swap provider if needed | Eng | Active |
| R-10 | Low adoption / unclear value | Med | High | **High** | Portfolio MVP validates with real data for JB; user interviews at 100 users; free public access; methodology as differentiator | Product | Active |
| R-11 | Multi-tenancy retrofit pain at Level 3 | Low→Med | High | **Med** | workspace_id on ownership-bearing parents now; documented backfill path; RLS-ready; decision recorded (ADR-009) | Architect | Mitigated by design |
| R-12 | Single-VPS failure causes downtime | Med | Med | **Med** | Managed DB separate failure domain; nightly dumps offsite; < 4 h RTO runbook; 99.5% (not 99.9%) promised honestly; vertical scale path | DevOps | Accepted with mitigation |
| R-13 | Storage growth beyond plan | Low | Med | **Low** | Retention policies (90d payloads, partition drops); monthly volume review; S3 promotion criteria | DevOps | Active |
| R-14 | Auth vendor (Supabase) outage or pricing change | Low | Med | **Med** | Standard JWT/JWKS verification means backend degrades to cached-public-data mode; user table maps by subject (portable); alternative vendors evaluated in ADR-008 | Eng | Active |
| R-15 | Dashboard framework decision delays work | Low | Med | **Low** | Decided: Next.js (closes Phase 0 Q5 / ARB R-14) | Eng | Closed |

## 2. Risks Closed by Amendment

| Prior ID | Risk | Resolution |
|----------|------|-----------|
| ARB R-09 | MVP scope exceeds capacity | Scope cut to Level 2; estimate in engineer-days; promotion criteria |
| ARB R-10 | Incorrect metrics from formula errors | Methodology doc + corrected AC-3.2 + property tests |
| ARB R-12 | Operational overload (K8s+Temporal+NATS+Redis+TimescaleDB) | All deferred with measurable triggers |
| ARB R-13 | Event schema evolution breaks consumers | Events in-process for MVP; names/payloads versioned for future transport |
| ARB R-15 | No registration = no growth | Registration via managed auth in MVP |

## 3. Watchlist (Medium/Low, reviewed monthly)

- Open-Meteo licensing change (A-01) — monitor quarterly.
- METMalaysia publishes an API (would upgrade observation quality) — opportunity, not risk.
- Timezone/DST display confusion in user feedback.
- CSV export misuse perceived as data redistribution (ToS adjacency).
- Payload volume disk growth faster than model (linked to R-13).

## 4. Risk Governance

- Owner reviews each Active risk monthly; status changes recorded in this file's history.
- Any new Critical risk triggers an amendment-style mini-review.
- Gates (R-02, R-03) are launch-blocking and tracked as dependencies D-05/D-06.
