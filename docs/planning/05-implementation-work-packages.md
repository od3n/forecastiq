# ForecastIQ — Implementation Work Packages (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: Prompt §"Required implementation work packages"; `docs/planning/04-phase-1-estimate.md`; `docs/delivery/05-implementation-sequence.md`

27 independently reviewable work packages. Common quality gate for all: testing strategy §5 (unit coverage, contract tests where touched, lint clean, OpenAPI drift clean, golden path green). Estimates in engineer-days (low–high). **Do not implement until the architecture report issues GO for the specific package.**

---

## WP-01: Repository and Development Environment ⭐ (Recommended First Package — full definition in architecture report §14)

| Attribute | Specification |
|-----------|---------------|
| Objective | Bootstrap the monorepo so every subsequent package lands in a working, tested, deployable skeleton |
| Scope | Repo layout (delivery doc 01); Makefile; docker-compose (PG16 + app + volume); Go module + toolchain; golangci-lint config incl. depguard module rules; `.env.example`; README pointing to docs/; hello-world module boundary proof (two packages with interface); basic CI (lint + unit + build) |
| Dependencies | None |
| Modules | platform (skeleton), repo root |
| DB changes | None (compose PG for future) |
| API changes | None |
| Tests | CI green; compose-up smoke test script |
| Observability | Structured logging skeleton (slog config) |
| Security | gitleaks pre-commit + CI; .gitignore (.env, payloads) |
| Acceptance | `make dev-up` → app serves /healthz against compose PG; lint zero warnings; CI pipeline green on first PR |
| Estimate | 2–3 d |
| Risks | Toolchain churn (low) |
| Rollback | N/A (greenfield) |
| Commit slices | (1) repo skeleton + Makefile + compose; (2) lint/depguard config + CI workflow; (3) logging skeleton + healthz + smoke test |

## WP-02: Database Foundation and Migrations

| Attribute | Specification |
|-----------|---------------|
| Objective | Full schema from table-design doc, migration tooling, immutability triggers, partitions, seed |
| Scope | golang-migrate setup; migrations 0001..000N (all 18 tables per `docs/data/03-table-design.md`); enums; triggers (immutability, updated_at); initial partitions (current + 3 months); seed migration (system workspace, 2 providers, default config); migration dry-run CI job |
| Dependencies | WP-01 |
| Modules | All (schema ownership boundaries established) |
| DB changes | Entire schema |
| Tests | Migration up/down (reversible); trigger raise tests; partition existence test; seed idempotency (run twice) |
| Acceptance | Fresh DB migrates to latest; all constraints/triggers verified by integration tests; CI dry-run green |
| Estimate | 3–4 d |
| Risks | Partitioned-PK FK composite subtleties (medium — spike early in package) |
| Rollback | Down migrations in dev; forward-fix in prod |
| Commit slices | (1) catalog tables + triggers; (2) collection tables + partitions; (3) analysis + scheduler + identity + audit; (4) seed + CI dry-run |

## WP-03: Identity and Workspace Foundation

| Attribute | Specification |
|-----------|---------------|
| Objective | User provisioning, JWT verification (JWKS), API keys, roles, audit recorder |
| Scope | identity module (domain/application/persistence); JWKS verifier adapter (cached, rotation-tolerant); provision-on-first-use; API key create/list/revoke (argon2id, shown once); audit module (recorder + reader); dev-mode auth for local (build-tag excluded from release) |
| Dependencies | WP-02 |
| Modules | identity, audit |
| DB changes | None (schema from WP-02) |
| API | `GET/PATCH /me`, `/api-keys` CRUD (handlers land in WP-15; use cases + tests here) |
| Tests | JWKS verification (valid/expired/wrong-iss/unknown-kid); provisioning idempotency; key hash never returned; audit emission per action |
| Security | Serializer-level credential exclusion; role from DB not JWT |
| Acceptance | Authenticated use-case flows green; audit rows for all identity actions |
| Estimate | 3–4 d |
| Risks | Supabase local-dev friction (mitigated by dev-mode signer) |
| Rollback | Feature-flagged routes (not wired until WP-15/19) |
| Commit slices | (1) audit module; (2) JWKS verifier + provisioning; (3) API keys; (4) use-case tests |

## WP-04: Location Management

| Attribute | Specification |
|-----------|---------------|
| Objective | Location CRUD with dedup (BR-LOC-01), timezone validation, soft-delete |
| Scope | catalog module location aggregate; haversine dedup check (0.05°) with override; IANA timezone validation; status lifecycle; audit integration |
| Dependencies | WP-03 (audit), WP-02 |
| Modules | catalog |
| API | Location use cases (handlers in WP-15) |
| Tests | Dedup boundary (exactly 0.05°), override flag, inactive location blocks collection eligibility, audit |
| Acceptance | BR-LOC-01..03 behaviors proven by tests |
| Estimate | 2 d |
| Rollback | N/A (additive) |
| Commit slices | (1) aggregate + repository; (2) dedup + validation; (3) tests |

## WP-05: Provider Adapter Framework

| Attribute | Specification |
|-----------|---------------|
| Objective | The adapter port + shared collection pipeline (validate → checksum → payload → decompose → normalize → store) provider-agnostic |
| Scope | `ForecastProviderAdapter` port; schema validation helper; checksum + gzip payload store adapter (scheme-prefixed keys); condition taxonomy mapper (v1 tables); collection use case (tx: collection + snapshots + event); dedup rules (snapshot uniqueness + collection-level); error_code classification (FC-13); circuit breaker (persistent, catalog-owned) |
| Dependencies | WP-02, WP-04 |
| Modules | collection, catalog (circuits) |
| Tests | Pipeline with stub adapter (success/partial/drift/dedup/replay); checksum verification; circuit transitions (5 failures → open → half-open → closed); condition unmapped counter |
| Security | Payloads never logged; credential_ref resolution from env |
| Acceptance | Full collection pipeline green with a fake provider; idempotent re-execution proven |
| Estimate | 4–5 d |
| Risks | Core design package — highest review attention |
| Rollback | N/A (not yet scheduled) |
| Commit slices | (1) ports + payload store; (2) collection use case + dedup; (3) circuit breaker; (4) condition mapper + tests |

## WP-06: First Forecast Provider (Open-Meteo)

| Attribute | Specification |
|-----------|---------------|
| Objective | Open-Meteo adapter against real API + recorded fixtures |
| Scope | Adapter (schema v1, 168-period hourly array); UTC normalization; fixture capture script + committed fixtures; contract test matrix (contract testing doc §1.2); attribution config seed |
| Dependencies | WP-05 |
| Tests | Full contract matrix; real-API smoke (manual, documented) |
| Acceptance | Fixture suite green; one live collection into local DB verified |
| Estimate | 2–3 d |
| Risks | API quirks (R-01) — fixture accumulation mitigates |
| Commit slices | (1) adapter + happy path; (2) edge/drift fixtures; (3) contract suite |

## WP-07: Second Forecast Provider (OpenWeather)

| Attribute | Specification |
|-----------|---------------|
| Objective | OpenWeather OneCall adapter (ToS-gated: D-05 evidence collected during this package) |
| Scope | Adapter (48-period hourly); rate-budget enforcement (daily counter); 401/429 handling fixtures; swap-path documentation (Tomorrow.io slot) |
| Dependencies | WP-06 (pattern proven) |
| Tests | Contract matrix incl. rate-limit and auth-failure fixtures |
| Acceptance | Contract suite green; budget enforcement test (429 → pause) |
| Estimate | 2–3 d |
| Risks | ToS review outcome (gate for public launch, not for code) |
| Commit slices | (1) adapter; (2) budget + error fixtures; (3) contract suite |

## WP-08: Forecast Scheduler and Collection Operations

| Attribute | Specification |
|-----------|---------------|
| Objective | Slot-based scheduler (ADR-005), run history, manual trigger, replay |
| Scope | scheduler module (generation, SKIP LOCKED claims, leases, backoff, watchdog); schedule_runs; `POST /admin/collections/trigger` (409 circuit / 429 budget guards); `POST /admin/collections/{id}/replay` (checksum verify, quarantine); `GET /forecast-collections` (admin); graceful shutdown drain; `--mode` flag |
| Dependencies | WP-05, WP-06, WP-04 |
| Tests | Concurrent claim test (2 workers, no double); lease expiry re-claim; missed-slot detection; replay idempotency; trigger guards |
| Observability | Scheduler metrics (claimed/missed/lag/duration) |
| Acceptance | Hourly cycle runs unattended for 48 h in dev; double-fire produces zero duplicates |
| Estimate | 4–5 d |
| Risks | Lease tuning (5 min default, measured) |
| Rollback | Scheduler disabled by mode flag; API unaffected |
| Commit slices | (1) slots + claims; (2) dispatch + run history; (3) trigger/replay endpoints; (4) shutdown + watchdog |

## WP-09: Observation Source Adapter

| Attribute | Specification |
|-----------|---------------|
| Objective | Open-Meteo Historical adapter with provenance typing |
| Scope | Adapter (2 h window); observation_type resolution (API-exposed provenance, documented reanalysis default); range validation (OC-04); correction detection (value-diff beyond ε); fixtures |
| Dependencies | WP-05 (shared validation) |
| Tests | Provenance tagging; suspect flagging; correction detection; dedup |
| Estimate | 2 d |
| Commit slices | (1) adapter + validation; (2) correction detection + fixtures |

## WP-10: Observation Collection

| Attribute | Specification |
|-----------|---------------|
| Objective | Scheduled observation pipeline + correction cascade initiation |
| Scope | Observation slots (:05); storage tx + `observation.collected`/`observation.corrected` events; supersession update (the one permitted mutation); freshness gauge |
| Dependencies | WP-09, WP-08 (scheduler) |
| Tests | Window backfill dedup; correction → superseded link + event; suspect exclusion downstream |
| Acceptance | 48 h unattended observation collection in dev |
| Estimate | 2–3 d |
| Commit slices | (1) collection job; (2) correction flow + events |

## WP-11: Matching Engine

| Attribute | Specification |
|-----------|---------------|
| Objective | Deterministic exact-hour matching per BR-MATCH-01..06 |
| Scope | analysis matching (batch, chunked 5K); candidate selection (provenance rank, corrected preference, tiebreak); rematch on correction event; backlog gauge |
| Dependencies | WP-08, WP-10 |
| Tests | Selection determinism (property: shuffled candidates → same choice); rematch creates new pairs (old retained); suspect exclusion; idempotent re-run |
| Acceptance | Matching batch over seeded month of data < 2 min; all BR-MATCH rules tested |
| Estimate | 3–4 d |
| Commit slices | (1) batch + selection; (2) correction rematch; (3) properties |

## WP-12: Pair-Level Evaluation

| Attribute | Specification |
|-----------|---------------|
| Objective | In-memory pair computations (errors, classification, Brier, weights) |
| Scope | Evaluation kernel (pure functions); observation-quality weighting; eligibility per variable; all 5 test vectors exact |
| Dependencies | WP-11 |
| Tests | TV-1..TV-5 exact; weighting (TV-5); classification boundaries (0.5 prob, 0.1 mm) |
| Estimate | 2–3 d |
| Commit slices | (1) continuous; (2) categorical + probabilistic; (3) vectors |

## WP-13: Aggregated Metrics

| Attribute | Specification |
|-----------|---------------|
| Objective | AccuracyMetric rows with CIs, null rules, coverage/reliability |
| Scope | Aggregation batch (per cell-period); weighted formulas; Wilson CIs; zero-denominator → null; coverage (schedule-derived denominator) + reliability (FC-13 classified); supersede on recompute; daily/weekly/monthly periods |
| Dependencies | WP-12 |
| Tests | Null rules (TV-3 pattern); CI sanity (coverage probability simulation); supersede links; byte-identical recompute (property 11) |
| Acceptance | Metrics for seeded month match hand-computed reference |
| Estimate | 4–5 d |
| Commit slices | (1) continuous aggregation + CIs; (2) categorical + coverage/reliability; (3) recompute/supersede |

## WP-14: Provider Ranking

| Attribute | Specification |
|-----------|---------------|
| Objective | ProviderRanking rows per methodology §6–7 |
| Scope | Cohort normalization (ε guard, null redistribution); weights (default + custom hash); coverage penalty + outranking rule; statuses (30/10/7-day); CI propagation; tie grouping; horizon profiles; atomic publication tx; worked-example integration test (ADR-010) |
| Dependencies | WP-13 |
| Tests | Worked example exact; penalty monotonicity (property 9); composite bounds (property 10); tie grouping; BR-RANK-04 outranking rule |
| Acceptance | Methodology §8 table reproduced exactly in integration test |
| Estimate | 4–5 d |
| Commit slices | (1) normalization + weights; (2) penalty + statuses; (3) ties + profiles; (4) worked example test |

## WP-15: Dashboard Query APIs

| Attribute | Specification |
|-----------|---------------|
| Objective | Public catalog + analysis read endpoints with full envelope conventions |
| Scope | Envelope assembly package (metadata/freshness/provenance/attribution/warnings/pagination); LRU + ETag middleware; `GET /rankings` (+observation_context), `/rankings/methodology`, `/accuracy/summary` (both modes + collection_window), `/accuracy`, `/locations`, `/providers` (+adapter_version/collecting_since); cursor pagination; rounding; OpenAPI generation + drift CI; partial-result assembly |
| Dependencies | WP-14, WP-03 |
| Tests | Every endpoint: contract + envelope + auth class + pagination + ETag/304 + partial fixture; response size assertions |
| Acceptance | Screen API contracts (doc 01) fully satisfied; OpenAPI drift gate green |
| Estimate | 5–6 d |
| Commit slices | (1) envelope + middleware; (2) rankings + methodology; (3) summary + trends; (4) catalog endpoints; (5) partials + sizes |

## WP-16: Forecast Evolution API (Forecast-vs-Actual)

| Attribute | Specification |
|-----------|---------------|
| Objective | `GET /forecast-comparison` (public, bounded, C-19) |
| Scope | Issuance selection (DR-02 nearest-shorter-horizon rule); day metrics in-memory (≤ 48 pairs); observation gaps as absences; per-series freshness + provenance; date-in-location-tz interpretation |
| Dependencies | WP-08, WP-10 (data), WP-15 (envelope) |
| Tests | DR-02 selection; gap rendering data; day metrics vs. evaluation kernel (same functions); size bound |
| Estimate | 2–3 d |
| Commit slices | (1) endpoint + selection; (2) day metrics + provenance |

## WP-17: Accuracy Analytics API

| Attribute | Specification |
|-----------|---------------|
| Objective | Trend bucketing (tz-aware), provider grid mode completion |
| Scope | `GET /accuracy` aggregation/tz bucketing (date_trunc post-scan); hollow points (sample_count per bucket); 365-d bound enforcement |
| Dependencies | WP-13, WP-15 |
| Tests | Bucketing across DST boundary (tz echo); bound rejection; hollow points |
| Estimate | 2 d |
| Commit slices | (1) bucketing; (2) bounds + tests |

## WP-18: Collection-Health API and Admin Operations

| Attribute | Specification |
|-----------|---------------|
| Objective | S-10/S-13 backend: health assembly, provider/location admin, user management, audit read |
| Scope | `GET /admin/health` (cells + circuits + next_scheduled_at + observation_collector + system section incl. statfs + backup status file); provider status/config endpoints (credential never echoed); user management endpoints (lockout guards, Supabase propagation); `GET /admin/audit-events`; recompute endpoint |
| Dependencies | WP-08, WP-03 |
| Tests | Health assembly < 200 ms under poll simulation; self-lockout 409s; credential absence grep; Supabase-failure 502 path |
| Estimate | 4–5 d |
| Commit slices | (1) health assembly; (2) provider/location admin; (3) users + audit; (4) recompute |

## WP-19: Authentication and Authorization Integration

| Attribute | Specification |
|-----------|---------------|
| Objective | Production auth wiring: middleware chain, Supabase project config, webhook ingestion, GDPR flows |
| Scope | RequireAuth/RequireRole/RequireScope middleware on all routes (matrix-driven); Supabase project hardening config (password policy, verification, rotation) documented + applied; auth webhook receiver (signed); `POST /me/export` + `DELETE /me` (Supabase admin propagation); auth audit events |
| Dependencies | WP-03, WP-15/18 (routes exist) |
| Tests | Full authorization matrix as integration tests (public/user/admin × every endpoint); webhook signature rejection; export/delete flows |
| Security | This package IS the security wiring — threat-model tests land here |
| Estimate | 3–4 d |
| Commit slices | (1) middleware + route wiring; (2) webhooks; (3) GDPR flows; (4) matrix tests |

## WP-20: Frontend Foundation

| Attribute | Specification |
|-----------|---------------|
| Objective | Next.js static-export app shell with API client, auth SDK, design tokens, state infrastructure |
| Scope | Next.js init (static export); generated API client (OpenAPI); Supabase SDK auth flow (S-08); design tokens per design system spec; envelope/warnings/freshness rendering primitives; error-boundary + request_id display (S-15); CSV export utility (conventions §5); axe-core CI from first screen |
| Dependencies | WP-15 (API surface) |
| Tests | Client generation contract; auth flow e2e (dev Supabase); a11y baseline |
| Estimate | 4–5 d |
| Commit slices | (1) app shell + tokens; (2) API client + envelope primitives; (3) auth screens; (4) export utility + a11y CI |

## WP-21: Core MVP Screens

| Attribute | Specification |
|-----------|---------------|
| Objective | All 15 screens with all 19 states per binding contracts |
| Scope | S-01..S-14 per screen specifications + IA doc; charts (CI bands, gaps, hollow points, keyboard nav); admin sections; settings; onboarding (localStorage dismissal); URL-synced filters; state fixtures for every state; Playwright critical flows |
| Dependencies | WP-20, WP-15..18 |
| Tests | State-contract matrix (fixture per state); axe-core zero critical; keyboard/SR manual pass on chart screens; ≤ 2 requests per screen verified |
| Acceptance | Reconciliation quality gates (report §"Quality Gate Verification") all pass |
| Estimate | 12–15 d |
| Risks | Largest package; state discipline (mitigated by contracts-as-acceptance) |
| Commit slices | Per screen group: (1) S-01/S-02; (2) S-03/S-04; (3) S-05/S-06; (4) S-07/S-08/S-09; (5) S-10/S-11; (6) S-12/S-13/S-14; (7) S-15 + polish |

## WP-22: Observability

| Attribute | Specification |
|-----------|---------------|
| Objective | Full metric catalog, structured logs, health endpoints, Grafana dashboards + alerts |
| Scope | All metrics per observability doc §3 (RED, collection, engine, scheduler, runtime); log event registry; grafana-agent config; 3 dashboards as code; alert rules A1–A17 as code; uptime checks; SLO recording rules |
| Dependencies | WP-08 (scheduler metrics surface) |
| Tests | Metric presence integration test (scrape /metrics, assert names); log field assertions (no secrets) |
| Estimate | 3–4 d |
| Commit slices | (1) metric instrumentation; (2) log registry + sanitization; (3) dashboards + alerts |

## WP-23: CI/CD and Deployment

| Attribute | Specification |
|-----------|---------------|
| Objective | Full pipeline per CI/CD doc; production deploy working end-to-end |
| Scope | All PR jobs; main pipeline (build, sign, deploy, smoke); Terraform (DNS + DB); bootstrap script; Caddyfile + systemd; rollback procedure rehearsed; Pages setup |
| Dependencies | WP-01 (skeleton pipeline exists) |
| Tests | Pipeline itself: deploy to scratch VPS in CI; rollback drill < 5 min |
| Estimate | 4–5 d |
| Commit slices | (1) PR jobs complete; (2) Terraform + bootstrap; (3) deploy + smoke; (4) rollback drill |

## WP-24: Backup and Recovery

| Attribute | Specification |
|-----------|---------------|
| Objective | Nightly dumps, offsite sync, monthly restore test, status file → admin health |
| Scope | backup.sh + restore-test.sh (spec in backup doc); cron; rclone B2; backup status file consumed by /admin/health; PITR procedure validated with vendor |
| Dependencies | WP-23 |
| Tests | Restore test run green; status file appears in health payload; alert A10/A11 firing test |
| Estimate | 2 d |
| Commit slices | (1) backup script + cron; (2) restore test + health integration |

## WP-25: Security Hardening

| Attribute | Specification |
|-----------|---------------|
| Objective | Pre-launch security pass per threat model |
| Scope | Security headers (Caddy + app); CORS final allowlist; rate-limit tuning; request size limits; dependency scans clean; secret rotation drill; OWASP checklist (NFR-SEC14); pen-test-style self-assessment against threat model (each threat → test exists); privacy policy + terms pages |
| Dependencies | WP-19 |
| Tests | Threat-model test matrix 100% covered; govulncheck/Trivy zero critical |
| Estimate | 3–4 d |
| Commit slices | (1) headers/CORS/limits; (2) threat matrix completion; (3) rotation drill + checklist |

## WP-26: Performance and Reliability Validation

| Attribute | Specification |
|-----------|---------------|
| Objective | All PT scenarios green; reliability suite green; NFR-S01 evidence |
| Scope | k6 scenarios PT-1..PT-8; synthetic seeder; reliability tests (timeout, rate limit, malformed payload, duplicate job, late observation, worker restart, DB reconnect); baseline register populated; load test at 2× volume |
| Dependencies | WP-21 (full system) |
| Tests | Threshold gates per performance doc §5 |
| Estimate | 3–4 d |
| Commit slices | (1) seeder + k6 scripts; (2) reliability suite; (3) baseline runs + report |

> **DRB note (2026-07-27)**: WP-26 was accepted at **slice 1 (scaffold)** only —
> k6 PT-1/PT-2/PT-6 (enforcing thresholds), a request-path reliability slice,
> and a deterministic seeder that estimates volumes but does not yet write
> rows. The remainder is tracked as **WP-26b** below.

## WP-26b: Performance Validation — Completion

| Attribute | Specification |
|-----------|---------------|
| Objective | Complete WP-26: all PT scenarios green + NFR-S01 evidence |
| Scope | PT-3 (ingestion burst), PT-4 (analysis batch, NFR-P06), PT-7 (2× volume, NFR-S01 Level-1 exit gate), PT-8 (Lighthouse); the 5 fault-injection reliability scenarios (provider timeout, duplicate job, late observation, worker restart via `docker compose restart app`, DB reconnect via `stop/start db`); functional seeder DB writes (`--estimate-only` currently gates the scaffold); populate the baseline register (§6); run the 2× volume load test; wire the weekly k6+reliability job into `.github/workflows/scheduled.yml` |
| Dependencies | WP-26 (scaffold, accepted) |
| Tests | Threshold gates per performance doc §5; baseline numbers recorded |
| Commit slices | (1) seeder DB generation; (2) PT-3/4/7/8 + reliability fault injection; (3) baseline runs + scheduled wiring |

## WP-27: Documentation and Demo Preparation

| Attribute | Specification |
|-----------|---------------|
| Objective | Launch-ready documentation, demo environment, methodology page content |
| Scope | README refresh; runbooks verified against reality; methodology page content review (S-06 serves registry); demo script (portfolio walkthrough); attribution verification (BR-ATTR-01 every surface); launch checklist execution (D-05, D-06 gates) |
| Dependencies | WP-26 |
| Estimate | 2–3 d |
| Commit slices | (1) docs refresh; (2) demo + checklist |

---

## Summary Table

| WP | Name | Days | Phase |
|----|------|------|-------|
| 01 | Repository + dev env ⭐ | 2–3 | Foundation |
| 02 | DB foundation | 3–4 | Foundation |
| 03 | Identity + workspace | 3–4 | Collection core |
| 04 | Locations | 2 | Collection core |
| 05 | Adapter framework | 4–5 | Collection core |
| 06 | Open-Meteo adapter | 2–3 | Collection core |
| 07 | OpenWeather adapter | 2–3 | Provider 2 + obs |
| 08 | Scheduler + collection ops | 4–5 | Collection core |
| 09 | Observation adapter | 2 | Provider 2 + obs |
| 10 | Observation collection | 2–3 | Provider 2 + obs |
| 11 | Matching engine | 3–4 | Analysis |
| 12 | Pair-level evaluation | 2–3 | Analysis |
| 13 | Aggregated metrics | 4–5 | Analysis |
| 14 | Provider ranking | 4–5 | Analysis |
| 15 | Dashboard query APIs | 5–6 | API |
| 16 | FvA API | 2–3 | API |
| 17 | Accuracy analytics API | 2 | API |
| 18 | Health + admin APIs | 4–5 | API |
| 19 | AuthN/AuthZ integration | 3–4 | API |
| 20 | Frontend foundation | 4–5 | Dashboard |
| 21 | Core screens | 12–15 | Dashboard |
| 22 | Observability | 3–4 | Hardening |
| 23 | CI/CD + deployment | 4–5 | Foundation (interleaved) |
| 24 | Backup + recovery | 2 | Hardening |
| 25 | Security hardening | 3–4 | Hardening |
| 26 | Perf + reliability validation | 3–4 | Hardening |
| 27 | Docs + demo | 2–3 | Launch prep |
| | **Total** | **88–112** (+ 13–18 cross-cutting/spikes/buffer = 104–129, matching estimate §2) | |

## Cross-Reference

- Sequence + milestones: `docs/delivery/05-implementation-sequence.md`
- Estimate reconciliation: `docs/planning/04-phase-1-estimate.md`
- First package GO decision: `docs/reviews/05-phase-1-architecture-report.md` §14/§17
