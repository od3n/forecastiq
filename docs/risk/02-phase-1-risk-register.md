# ForecastIQ — Phase 1 Risk Register

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: Prompt §"Architecture risks"; supersedes-and-extends `docs/risk/01-risk-register.md` (which remains the Phase 0 baseline; new IDs continue the sequence)

Severity = Probability × Impact. Review cadence: monthly with the Phase 0 register, or on trigger.

---

## 1. Critical and High Risks (Phase 1 additions)

| ID | Risk | P | I | Severity | Mitigation | Detection | Owner | Residual |
|----|------|---|---|----------|-----------|-----------|-------|----------|
| R-16 | Provider ToS/licensing blocks publication of derived metrics (extends R-02) | Low | Critical | **High** | D-05 gate before public launch; attribution everywhere; store normalized + derived (not raw redistribution); swap path (Tomorrow.io); legal review of OpenWeather terms for non-commercial storage+display | Gate process; terms-change watchlist (quarterly) | Operator | Gate-blocked until D-05 closes |
| R-17 | Observation quality inadequate — reanalysis error misranks providers in tropical convection (extends R-03) | Med | High | **High** | D-06 quality spike vs. METMalaysia bulletins pre-launch; provenance weighting (0.8) + prominent disclosure; rankings never presented as ground truth; NOAA path for US | Spike results; user feedback; literature | Eng | Managed via disclosed uncertainty |
| R-18 | Inaccurate matching (wrong observation paired) silently corrupts metrics | Low | Critical | **High** | Deterministic total-order selection (no ambiguity); match_rule + time_delta stored per pair; observation_id lineage; property tests (determinism, permutation invariance); rematch only via correction events | Pair audit via lineage queries; spot-check tooling (admin) | Eng | Low (deterministic + tested) |
| R-19 | Incorrect statistical formulas in aggregation/ranking (extends R-07 to Phase 1 scope) | Low | Critical | **High** | Canonical methodology doc; 5 vectors + 11 properties in CI (1K cases; 10K nightly); worked example as integration test (byte-exact); formula-change governance (version bump + review checklist) | CI gates; recomputation reproducibility checks | Eng | Low |
| R-20 | Data growth exceeds model (DB tier jumps, cost) | Low | Med | **Med** | Growth model with explicit assumptions (data doc 06); monthly volume review; retention jobs; supersede-purge policy; promotion triggers (TimescaleDB/S3) | Grafana volume panels; monthly review | DevOps | Low |
| R-21 | Scheduler failure goes unnoticed (silent collection stop) | Low | High | **Med** | Watchdog metric (missed slots); lease expiry self-healing; freshness degradation visible in UI (BR-FRESH); alert A8; S-10 next_scheduled_at | A8 alert; freshness states | Eng | Low |
| R-22 | Duplicate collection despite safeguards (double billing of rate budget, data noise) | Low | Med | **Low** | Three layers: SKIP LOCKED claims, collection-level dedup (model run time), snapshot uniqueness + ON CONFLICT; concurrency integration tests | Dedup counter (snapshots_deduplicated); budget monitoring | Eng | Low |
| R-23 | Provider schema change breaks adapter silently (extends R-01) | Med | High | **High** | Schema validation per row; > 50% invalid → failed + schema_drift alert (A6); contract fixtures accumulate drift history; replay for recovery (90 d window); unmapped-condition alert (A14) | A6/A14 alerts; collection status monitoring | Eng | Med (bounded by alerting + replay) |
| R-24 | Provider rate limits constrain collection at > 10 locations (extends R-09) | Med | Med | **Med** | Hourly cadence with 4× headroom at MVP scale; token bucket + daily budget; trigger endpoint 429 guard; location count is scope decision | Budget metrics; 429 rate | Eng | Low at MVP scale |
| R-25 | Stale observations misrepresent current accuracy (user trust) | Med | Med | **Med** | Freshness states server-computed + always displayed (BR-FRESH); staleness banners; rankings batch-stable during gaps | Freshness gauges; UI contract tests | Eng | Low (honesty by design) |
| R-26 | Ranking misuse (headline score without context) damages credibility | Low | High | **Med** | BR-RANK-06 (breakdown always one interaction away); statuses + sample counts + CIs always shown; methodology page; tie grouping; provisional labels | UI contract tests; methodology review | Product | Low |
| R-27 | Auth vendor outage blocks authenticated access (extends R-14) | Low | Med | **Med** | Public data unaffected (AUTH-08); cached public reads continue; JWKS cache tolerates 15 min; subject-mapping portability | Vendor status; 401 rate monitoring | Eng | Low |
| R-28 | Database outage loses in-flight collections | Low | Med | **Low** | Managed PITR (RPO < 1 h); slot re-claim after outage; idempotent re-collection; providers retain data for natural backfill | A1/A2 alerts; readyz | DevOps | Low |
| R-29 | Raw payload leakage (licensing exposure) | Low | High | **Med** | No file-serving route (surface absent); volume not web-accessible; admin sees key prefix only; 90 d retention; threat model §5 | Filesystem audits; route absence asserted in tests | Eng | Low |
| R-30 | Operational overload for single operator (extends R-05/R-06) | Med | High | **High** | Runbooks for all critical alerts (every alert has a procedure); monthly drills (rollback, restore); admin UI self-service (no log-system queries needed); managed services absorb DB ops | Alert actionability reviews; drill completion | Eng/DevOps | Med (inherent to team size; mitigated by design) |
| R-31 | Cost growth beyond $500 ceiling | Low | Med | **Low** | Expected $42–47/mo; largest drivers identified (DB tier, provider tiers); budget alerts; growth estimate at 10× still ~$160 | Vendor billing alerts; monthly review | Operator | Low |
| R-32 | Single-maintainer knowledge concentration | High | Med | **High** | Complete architecture package (this set); ADRs for all material decisions; runbooks; onboarding target < 2 d (NFR-M08); sealed-envelope secrets procedure | Onboarding drill with second party (quarterly) | Eng | Med |
| R-33 | Migration complexity at promotion time (Redis/NATS/etc.) exceeds plan | Low | Med | **Low** | Seams built in Phase 1 (cache port, event seam, SKIP LOCKED, scheme-prefixed keys); each promotion ADR-supervised with measured trigger; migration paths documented per technology | Promotion review at trigger | Architect | Low |
| R-34 | Dashboard query performance degrades with data age | Low | Med | **Low** | Pre-computed projections for all aggregates; indexed access paths verified (Q-01..Q-11 + QX); PT-7 quarterly baselines; LRU/ETag absorption | PT runs; pg_stat_statements review | Eng | Low |
| R-35 | Location dedup (BR-LOC-01) bypassed under concurrent creates — check-then-insert TOCTOU (found DRB-WP04-001, reproduced) | Low | Med | **Med** | Remediation required in WP-04: serialize create tx (`pg_advisory_xact_lock` or SERIALIZABLE + 40001 retry) + concurrency integration test; duplicate rows detectable via proximity query for manual cleanup | Re-review gate (WP-04 cannot be Accepted open); admin audit trail (`location.create` rows) | Eng | **Mitigated 2026-07-23** — `pg_advisory_xact_lock` serializes create tx; `TestAPI_ConcurrentDuplicateCreates` proves 1 row under 6 concurrent creates (real PostgreSQL). Residual: WP-04 acceptance still gated on TC-04 (pushed-branch CI) per re-review. |

## 2. Gates Carried Forward (launch-blocking)

| Gate | Risk | Status |
|------|------|--------|
| D-05: OpenWeather ToS validation | R-02/R-16 | Open — blocks public launch of attributed provider displays |
| D-06: Observation quality spike | R-03/R-17 | Open — blocks ranking publication at launch |

## 3. Watchlist (Phase 1 additions)

- Open-Meteo Historical provenance exposure changes (affects observation_type resolution default).
- Supabase Auth webhook format changes (versioned; receiver validates).
- Cloudflare Pages build behavior changes (static export compatibility).
- Neon/Supabase tier pricing changes (cost model sensitivity).
- grafana-agent remote-write format changes.

## 4. Governance

- This register reviewed together with `docs/risk/01-risk-register.md` monthly.
- Any new Critical risk → amendment-style mini-review.
- Phase 1 risks with design-based mitigation are re-verified at the corresponding work package exit (mitigation actually implemented, not just designed).
