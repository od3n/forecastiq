# ForecastIQ — User Stories (Revised)

**Version**: 2.0 (Phase 0 Amendment)
**Status**: Authoritative
**Supersedes**: `docs/phase-0-business-analysis/08-user-stories.md`

Estimates use **engineer-days** (1 engineer-day = one focused 8-hour day). Story points
are abandoned: without a calibrated team velocity they were false precision
(amendment mandate). Ranges reflect unknowns; dependencies and risks per epic are in
`docs/planning/02-revised-mvp-estimate.md`.

---

## Epic 1: Forecast Collection (revised)

### US-1.1: Collect and decompose provider forecasts
**As the** operator **I want** hourly collection from Open-Meteo and OpenWeather, with
each API response stored as one ForecastCollection and decomposed into per-target-time
ForecastSnapshots **so that** we have immutable, lineage-complete multi-provider data.

**AC**: collection + snapshot records per domain model; raw payload gzipped +
checksummed; dedup on `(provider, location, issued_at, target_time)`; condition codes
mapped to canonical taxonomy; metrics emitted.
**Estimate**: 6–8 d | **Priority**: Critical

### US-1.2: Configure schedules
**As the** operator **I want** per-provider collection intervals stored in the DB,
effective next cycle **so that** I can balance freshness vs. rate limits.
**AC**: interval config; invalid values rejected; default hourly.
**Estimate**: 2 d | High

### US-1.3: Failure handling
**As the** operator **I want** backoff retries, circuit breaker, and provider-vs-system
failure classification **so that** outages don't lose data or pollute reliability stats.
**AC**: per FR FC-08..FC-13; alert on circuit open.
**Estimate**: 3–4 d | High

### US-1.4: Replay raw payloads
**As the** operator **I want** to replay a stored payload through the current adapter
**so that** adapter fixes can recover missed snapshots without duplicates.
**AC**: new collection created; snapshot dedup holds; audit event.
**Estimate**: 2 d | Medium

## Epic 2: Observation Collection

### US-2.1: Collect provenance-typed observations
**As the** operator **I want** hourly Open-Meteo Historical observations with
observation_type and quality flags **so that** comparisons rest on disclosed ground truth.
**AC**: per FR OC-01..OC-06; dedup constraint; suspect handling; correction records.
**Estimate**: 4–5 d | Critical

## Epic 3: Accuracy Analysis

### US-3.1: Match forecasts to observations
**As a** data engineer **I want** exact-hour UTC matching with provenance-priority
conflict resolution **so that** every compared pair is defensible.
**AC**: per BR-MATCH-*; lineage IDs stored; suspect excluded.
**Estimate**: 4–5 d | Critical

### US-3.2: Compute metrics with CIs
**As a** data engineer **I want** all methodology metrics computed with quality
weighting, null handling, and confidence intervals **so that** numbers are statistically
sound and reproducible.
**AC**: per methodology §4–5; test vectors pass; property tests pass.
**Estimate**: 6–8 d | Critical

### US-3.3: Rank providers transparently
**As a** user **I want** rankings with composite score, per-metric breakdown, sample
sizes, coverage penalty, statuses, and tie handling **so that** I know who is best and
how sure we are.
**AC**: per methodology §6–7; worked example reproduced in integration test; unranked
cells say "insufficient data".
**Estimate**: 4–5 d | Critical

### US-3.4: Accuracy trends
**As a** user **I want** metric trends over selectable periods with provider overlay
**so that** I can spot improving/degrading providers.
**AC**: daily/weekly/monthly aggregation; CSV export.
**Estimate**: 3 d | High

## Epic 4: REST API

### US-4.1: Core query API
**As a** developer **I want** documented endpoints for providers, locations, forecasts,
observations, accuracy, rankings with cursor pagination, request IDs, ETags, rate-limit
headers, and provenance fields **so that** I can build on ForecastIQ.
**AC**: per `docs/api/00-api-requirements.md`; OpenAPI published; contract check in CI.
**Estimate**: 8–10 d | Critical

### US-4.2: Idempotent mutations
**As a** developer **I want** `Idempotency-Key` on POSTs **so that** retries don't
create duplicates.
**AC**: same key → same response within 24 h; collisions handled.
**Estimate**: 2 d | High

### US-4.3: API keys
**As a** user **I want** scoped, rate-limited API keys shown once **so that** I can
grant least-privilege programmatic access.
**AC**: per FR AUTH-05; audit logged.
**Estimate**: 3 d | High

## Epic 5: Authentication (managed)

### US-5.1: Account lifecycle via Supabase Auth
**As a** visitor **I want** to register (email-verified), log in, reset/update password,
and log out **so that** I can use the platform securely.
**AC**: per FR AUTH-01..AUTH-04, AUTH-07; no passwords in our DB; brute-force
protection verified.
**Estimate**: 3–4 d | Critical

### US-5.2: Admin user management
**As the** operator **I want** to disable/delete accounts and export account data
**so that** I meet support and GDPR needs.
**AC**: per AUTH-09.
**Estimate**: 2 d | Medium

## Epic 6: Dashboard

### US-6.1: Overview + rankings
**As a** user **I want** the overview with ranking cards, freshness, sample sizes,
provenance, and methodology link **so that** I get the answer plus the evidence.
**AC**: per DB-01..DB-04; all states from screen inventory implemented.
**Estimate**: 8–10 d | Critical

### US-6.2: Forecast vs. actual + trends views
**As a** user **I want** overlay charts and trend charts with selectors and URL state
**so that** I can explore and share specific comparisons.
**AC**: per DB-05; loading/error/stale states.
**Estimate**: 5–6 d | High

### US-6.3: Onboarding, methodology page, export
**As a new** user **I want** first-use guidance and a readable methodology page; **as
any** user **I want** CSV export with metadata **so that** I understand and can take the
data with me.
**AC**: per DB-06, DB-07.
**Estimate**: 3–4 d | High

## Epic 7: Administration

### US-7.1: Manage providers/locations/schedules
**As the** operator **I want** admin CRUD with dedup checks and audit logging **so
that** the catalog stays clean.
**AC**: per ADMIN-01, ADMIN-02, ADMIN-04; BR-LOC-01 enforced.
**Estimate**: 4–5 d | Critical

### US-7.2: Health & recovery
**As the** operator **I want** collector health, freshness states, failed-slot retry,
replay, and recompute triggers **so that** I keep the pipeline trustworthy.
**AC**: per ADMIN-03, ADMIN-06.
**Estimate**: 3–4 d | High

### US-7.3: Users & audit
**As the** operator **I want** user management and audit log views **so that** I can
administer accounts and investigate.
**AC**: per ADMIN-05.
**Estimate**: 2 d | High

## Epic 8: Platform & Ops (new epic — previously implicit)

### US-8.1: Bootstrap, CI/CD, deploy
**As the** team **I want** repo bootstrap, Docker Compose dev env, GitHub Actions
pipeline, migration tooling, single-VPS deploy with < 5 min rollback **so that** we ship
safely from day one.
**Estimate**: 5–6 d | Critical

### US-8.2: Observability & runbooks
**As the** operator **I want** structured logs, `/metrics`, Grafana dashboards, alert
rules, and Day-2 runbooks (backup restore, disk full, key rotation, rollback) **so
that** operations are boring.
**Estimate**: 4–5 d | High

### US-8.3: Testing foundation
**As the** team **I want** unit + property-based formula tests, adapter contract tests
against recorded fixtures, and integration golden-path tests **so that** statistical
correctness is continuously verified.
**Estimate**: 4–5 d (spread across epics; foundation 2 d) | Critical

## Removed stories

- US-8.1-old (forecast change alerts) → Level 3 backlog.
- Heatmap view → Level 3 (location-comparison table in MVP).
- "10M snapshots" performance stories → replaced by NFR-S01 headroom validation.

## Story Map Summary (revised)

| Epic | Stories | Engineer-days (range) | MVP? |
|------|---------|------------------------|------|
| Forecast Collection | 4 | 13–16 | ✓ |
| Observation Collection | 1 | 4–5 | ✓ |
| Accuracy Analysis | 4 | 17–21 | ✓ |
| REST API | 3 | 13–15 | ✓ |
| Authentication | 2 | 5–6 | ✓ |
| Dashboard | 3 | 16–20 | ✓ |
| Administration | 3 | 9–11 | ✓ |
| Platform & Ops | 3 | 13–16 | ✓ |
| **Total** | **23** | **90–110 engineer-days** | |

See `docs/planning/02-revised-mvp-estimate.md` for one-engineer vs. two-engineer
calendar estimates, dependencies, and risks.
