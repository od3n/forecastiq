# ForecastIQ — Product Contract

**Version**: 1.0 (Phase 0 Amendment)
**Status**: Authoritative

The product contract is the set of promises ForecastIQ makes to users. Anything not
promised here is not owed. This document exists so that UI, API, and documentation
decisions can be checked against explicit commitments instead of invented ad hoc.

---

## 1. Core Promises

| # | Promise | Enforcement mechanism |
|---|---------|----------------------|
| PC-01 | Every forecast we store is preserved exactly as collected (immutability), with lineage to the raw provider payload while retained. | DB immutability triggers; checksummed payloads; lineage chain |
| PC-02 | Every published number shows how it was computed: methodology version, weights, sample size, confidence interval where defined. | API schema requirements; UI methodology panel; BR-RANK-01/06 |
| PC-03 | We never publish a ranking from insufficient data. Below thresholds you see "provisional" or "insufficient data", never a fabricated order. | Ranking statuses; BR-RANK-02/09 |
| PC-04 | You always know what kind of observation our "reality" is based on (station / interpolated / reanalysis / provider-estimated) and its quality treatment. | observation_type exposure; provenance badges; quality weighting |
| PC-05 | You always know how fresh the data is. Stale data is labeled stale; we never silently serve old numbers as current. | Freshness states in every time-sensitive payload; BR-FRESH-01/02 |
| PC-06 | Providers get fair treatment: our own collection failures are distinguished from provider outages (coverage vs. reliability), and attribution is always given. | Collection error classification; attribution fields; BR-ATTR-01 |
| PC-07 | The API contract is stable within v1: breaking changes only in a new major version with ≥ 6 months deprecation notice (`Sunset`/`Deprecation` headers). | OpenAPI governance; deprecation headers |
| PC-08 | Your account security follows managed-auth best practice: verified email, hashed credentials (never in our DB), token rotation, brute-force protection. | Supabase Auth + app-side policies (Blocker 6 decisions) |
| PC-09 | You can export the data you are viewing (CSV) with the same provenance and methodology metadata. | Export flow in UI; CSV includes metadata header rows |
| PC-10 | When something is broken, the UI says so — per component, per provider — rather than showing plausible-looking stale charts. | Error/partial-failure states per screen inventory |

## 2. Explicit Non-Promises (MVP)

| # | Non-promise | Honest statement shown instead |
|---|-------------|-------------------------------|
| NP-01 | Real-time weather ("what is the weather now") | We measure forecasts, we don't deliver weather. |
| NP-02 | Perfect ground truth | Observations carry provenance; tropical reanalysis has known uncertainty — disclosed. |
| NP-03 | Alerts/notifications | Deferred (Level 3). |
| NP-04 | 99.9% availability | 99.5% target; status visible via health endpoint. |
| NP-05 | Unlimited history on demand | Retention per BR-09; older raw payloads deleted (normalized data kept). |
| NP-06 | Multi-user organizations, shared workspaces, billing | Level 3. |
| NP-07 | Ranking significance beyond stated CIs | Overlapping CIs are shown as ties, not ordered. |

## 3. Contract Governance

Any change to a Core Promise requires the amendment process (like business rules) and
a review pass. Non-promises may graduate to promises only with matching scope/infra
decisions (e.g., NP-04 graduates only alongside the Level 3 infrastructure promotion).
