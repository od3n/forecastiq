# ForecastIQ — Phase 1 Architecture Report

**Board**: Phase 1 Architecture Team
**Version**: 1.0
**Date**: 2026-07-22
**Inputs**: All Phase 0 Amendment authoritative documents; ADR-001..012; UI↔Backend Reconciliation outputs (16 documents); Phase 1 architecture package (~51 documents, ADR-013..032)

---

## 0. Pre-Flight: Prior Blockers Resolved

Verified before design began: the Reconciliation Report closed **all 20 conflicts** (0 still blocked), issued **GO for Phase 1 Architecture**, and confirmed every quality gate. Phase 0 Amendment resolved ARB Blockers 1–6. Dependencies D-05 (OpenWeather ToS) and D-06 (observation quality spike) remain as **launch gates**, not architecture blockers — the pipeline is identical regardless of their outcome. **No unresolved conflict between authoritative documents required precedence arbitration in Phase 1.**

## 1. Executive Summary

ForecastIQ's Phase 1 architecture converts the approved product into a buildable design for 1–2 engineers: **one Go binary** (API + in-process scheduler + collection + comparison engine) + **one static Next.js dashboard on CDN** + **one managed PostgreSQL 16** + Caddy for TLS, at **~$42–47/month**.

It is appropriate because:
- **It matches the workload exactly**: ~50K snapshots/day (0.6 rows/s average), ≤ 100 concurrent users, dozens of jobs/day — a domain where vanilla PostgreSQL with monthly partitioning has 100× headroom and distributed infrastructure is pure liability.
- **It concentrates engineering effort on the product's actual risk surface**: statistical correctness (11 property invariants + byte-exact worked example in CI), data lineage (unbroken raw-payload → ranking chain), provider integration resilience (circuit breakers, schema-drift alerting, replay), and honest uncertainty presentation (freshness states, provenance, sample sizes on every number).
- **Every deferral has a measured trigger and a prepared seam** — Redis, NATS, Temporal, Kubernetes, TimescaleDB, S3, worker split, read replica: none is a dead end; none is adopted in anticipation.

## 2. Approved Architecture

| Concern | Decision | Reference |
|---------|----------|-----------|
| Frontend | Next.js static export on Cloudflare Pages; ≤ 2 API requests per screen; no BFF | ADR-013, ADR-018 |
| Backend | Go modular monolith (Gin); 8 internal modules with interface-only coupling (depguard-enforced) | ADR-001, module architecture doc |
| Worker | Same binary; `--mode=api\|worker\|all` flag; scheduler = Go ticker + SKIP LOCKED slot claims | ADR-005, ADR-013 |
| Database | PostgreSQL 16 managed (Neon); monthly declarative partitions; no TimescaleDB | ADR-004, ADR-029 |
| Scheduler | In-process, DB-backed slots, leases, watchdog; no Temporal | ADR-005, scheduling workflow |
| Authentication | Supabase Auth; JWKS verification; stateless API; app-issued API keys (argon2id) | ADR-008, ADR-017 |
| Hosting | Hetzner VPS + Caddy + Cloudflare + Grafana Cloud free tier | ADR-007, ADR-026 |
| Observability | Structured JSON logs + Prometheus `/metrics` + 3 dashboards + 17 alerts; request-ID correlation (no distributed tracing) | Observability doc |
| CI/CD | GitHub Actions; trunk-based; OpenAPI drift gates; < 5 min rollback | CI/CD doc |

## 3. Major Decisions

All material decisions are ADR-recorded. Phase 0 ADRs (001–012) remain authoritative; Phase 1 added ADR-013..032:

| ADR | Decision |
|-----|----------|
| 013 | One binary + static dashboard; mode flag for future split |
| 014 | Deterministic batch matching; append-only rematch |
| 015 | 30-min batch; pair evaluation in memory; persisted metrics |
| 016 | Rankings as stored immutable projections |
| 017 | Role middleware + object checks; no RLS at MVP |
| 018 | Domain endpoints + 2 screen endpoints; no BFF (ratifies board) |
| 019 | Volume-only payloads; scheme-prefixed keys (extends 011) |
| 020 | Redis deferred; LRU + ETag + stale-serving |
| 021 | Five versioned in-process events (implements 006) |
| 022 | UUIDv7 identifiers |
| 023 | Monorepo with enforced module boundaries |
| 024 | PITR + nightly dumps + monthly restore test; no payload backup |
| 025 | Direct observation rows; polled with 2 h backfill window |
| 026 | Hetzner + Neon + Cloudflare (specifies 007) |
| 027 | Short bounded transactions; no outbox |
| 028 | Stale-serving contract (labeled, reads-only) |
| 029 | Monthly partitions; DROP-based retention (implements 004) |
| 030 | Methodology registry as versioned config, not a table |
| 031 | No staging environment |
| 032 | Quality gates bind work-package completion |

Full decision log: `docs/reviews/06-phase-1-decision-log.md`.

## 4. Domain and Data Model

**Aggregates** (full spec: `docs/architecture/04-domain-architecture.md`): Workspace (system), User+APIKey, Location, Provider, ProviderConfiguration (+ProviderCircuit state), ForecastCollection+Snapshots (immutable children), Observation (corrections as new rows + supersession), MatchedEvaluation (immutable pairs), AccuracyMetric, ProviderRanking, ScheduleSlot/Run, AuditEvent, ExportJob.

**Lineage** (unbroken, testable): raw payload (key + SHA-256) → collection → snapshot → match (both FKs) → metric (reproducible set) → ranking (component references + versions). No in-place mutation anywhere in the pipeline; trigger-enforced immutability; supersession is the only link mutation.

**Schema**: 18 tables (prompt's full list evaluated; `observation_collections` and `methodology_versions` deliberately not created — reconciliation verdicts ratified in ADR-025/030; `workspace_memberships` deferred per ADR-009 with workspace_id columns ready). UUIDv7 PKs; composite PKs on partitioned tables; native enums; JSONB only for payload-shaped data (never filter targets).

## 5. Critical Workflows

Full sequences with Mermaid diagrams in `docs/workflows/01..06`:

1. **Forecast collection**: slot claim → circuit check → rate bucket → provider call (10 s timeout) → checksum-before-parse → gzip payload → schema validate → decompose array → normalize (UTC, units, condition taxonomy) → single tx (collection + snapshots ON CONFLICT DO NOTHING + circuit + event) → run history. Retries 1/2/4/8/16 s ±20% jitter, max 5; circuit opens at 5 consecutive failures, half-open 60 s, state persisted.
2. **Observation collection**: hourly at :05, 2 h backfill window (late publication self-heals), provenance tagging (reanalysis default documented), range validation → suspect, corrections detected by value-diff → new row + supersession + event.
3. **Matching**: deterministic exact-hour UTC; total-order candidate selection (corrected → provenance rank → top-of-hour proximity → id); uniqueness-guarded inserts; rematch = new pairs only.
4. **Evaluation & ranking**: 30-min batch (match → in-memory pair eval → AccuracyMetric rows with weighted CIs → cohort normalization → weights → coverage penalty → statuses → CI ties → atomic publication). Rolling recompute absorbs late/corrected data; supersede links preserve history.
5. **Scheduling**: SKIP LOCKED claims, 5-min leases, watchdog, run history; multi-instance-safe from day one.
6. **Backfill/reprocessing**: replay from stored payloads (checksum-verified, current adapter, new collection); observation-driven automatic recompute; methodology upgrades via versioned recompute.

## 6. API Architecture

34 endpoints (catalog §`docs/api/05-endpoint-catalog.md`): composition = domain endpoints + `/forecast-comparison` + extended `/accuracy/summary` (ADR-018); URL-versioned `/api/v1/`; unified envelope (data/metadata/freshness/provenance/attribution/warnings/pagination — include only where meaningful); RFC 7807 errors with 11-class taxonomy + `retryable` + `request_id`; partial results = HTTP 200 + warnings (affected omitted, never per-row errors); server-computed freshness (4 states, BR-FRESH thresholds); cursor pagination without total_count; idempotency keys on mutable POSTs; per-key/per-IP rate limits with headers; ETag/304 + per-class Cache-Control; CORS allowlist; OpenAPI 3.1 generated from code with CI drift + breaking-change gates.

Authorization: public set per AUTH-08 (rankings/accuracy/comparison/catalog reads); raw data user+; admin endpoints role-gated; object checks in use cases; role from DB (immediate disable effect). Full matrices tested per row.

## 7. Database and Performance

**PostgreSQL 16, no TimescaleDB** (ADR-004 ratified with Phase 1 workload numbers): ~52K snapshots/day, ~19 GB steady state at 2 y retention, validated to 100M rows via monthly partitioning + the documented index set (24 indexes, including the reconciliation's additive `rankings(location_id, horizon_minutes, period_end DESC)` — verified added). All screen reads hit pre-computed projections via indexed scans (≤ 365 rows worst case); day metrics are the only query-time compute (≤ 48 pairs). Keyset pagination everywhere; no unbounded queries (required filters + limits, OpenAPI-enforced); no N+1 by design (provider-mode summary). Retention: partition drops (2 y/5 y) + bounded purges (matches 2 y, audit 1 y) + payload path-deletion (90 d). Growth model and cost: `docs/data/06-data-growth-and-cost-model.md`.

## 8. Security Posture

Authentication: Supabase Auth (no passwords stored by ForecastIQ); JWKS RS256 verification with rotation tolerance; refresh rotation with theft detection; mandatory email verification; immediate-effect disable (role from DB). Authorization: middleware → object checks → repository scoping (ADR-017); self-lockout guards; anti-enumeration; credentials structurally absent from serializers (not filtered — absent). Secrets: env-only, credential_ref indirection, gitleaks gates, rotation runbooks ≤ 30 min. Threat model covers all 16 mandated areas with mitigations + detection + residual ratings — notable architectural eliminations: no file-serving route (payload exposure surface absent), no user-supplied URLs fetched (SSRF surface absent), no string SQL. Audit: every auth event + admin mutation, same-transaction, immutable, 1 y, sanitized.

## 9. Reliability and Operations

SLIs/SLOs: 99.5% availability (honest single-VPS ceiling), p50 < 50 ms / p95 < 200 ms, ≥ 99% slot success, engine lag < 2 h, zero committed-data loss; error-budget policy with burn-rate alerts. Retries: backoff + jitter per policy table; circuit breakers per provider (persistent). Degradation matrix for every dependency (stale-cache serving with explicit staleness for reads; mutations fail hard). Backups: PITR (RPO < 1 h) + nightly dumps + weekly offsite + **monthly automated restore test visible in admin health**; VPS rebuild rehearsed < 4 h. Runbooks: deployment/rollback (< 5 min), provider failure (incl. schema-drift drill), database recovery (symptom → procedure map). 17 alert rules, each with a named response procedure.

## 10. Deployment and Cost

Environments: local (compose) / CI (ephemeral) / production — no staging (ADR-031). Production: Hetzner CX32 + Neon + Cloudflare Pages/DNS + Caddy + grafana-agent; deploys via GitHub Actions (build → migrate → restart → smoke); rollback = previous artifact < 5 min; migrations forward-only with expand-contract governance. IaC: Terraform for DNS + DB project; scripted VPS bootstrap; configs-as-code in repo. **Cost: ~$42–47/mo expected** (50%+ under the $150 target; ~$160 at 10× — still under the $500 ceiling); largest drivers: DB tier, provider API tiers.

## 11. Testing Strategy

Eight layers (unit → property → adapter contract → DB integration → API integration → e2e golden path → reliability → performance) + frontend (state-contract fixtures for all 19 states, axe-core from first screen). Formula coverage 100%: 5 test vectors + 11 property invariants (1K CI / 10K nightly cases) + worked example reproduced byte-exact as an integration test (ADR-010 mandate). Quality gates bind work-package completion (ADR-032); release gate adds reliability + performance + accessibility passes. Test data: synthetic only (production data never in test envs); JB canonical fixtures.

## 12. Scaling Path

Deferred with measured triggers and prepared seams (full register: `docs/architecture/10-scaling-and-evolution.md`): Redis (p95 > 150 ms read-dominated / shared rate limits) → worker split (batch > 10 min or API correlation) → read replica (write-path degradation) → NATS (engine lag > 15 min / second consumer) → Temporal (multi-step recovery / miss rate > 1%) → Kubernetes (≥ 3 services AND ≥ 4 engineers / SLAs > 99.5%). Plus: S3 (> 50 GB payloads), TimescaleDB (p95 > 200 ms at load), staging (second engineer). Every promotion is ADR-supervised with evidence; none is required for any earlier step to work.

## 13. Implementation Roadmap

27 work packages (`docs/planning/05-implementation-work-packages.md`), ordered in `docs/delivery/05-implementation-sequence.md`:

Foundation (wk 1–2): WP-01 repo/dev-env, WP-02 DB foundation, WP-23 pipeline skeleton → Collection core (3–7): WP-03..06, 08 → **M1** → Provider 2 + observations (7–9): WP-07, 09, 10 → Analysis (9–14): WP-11..14 → API (12–16): WP-15..19 → **M2** → Dashboard (15–21): WP-20, 21 → **M3** → Hardening (21–25): WP-22, 24..26 → Launch prep (25–26): WP-27 + D-05/D-06 gates → **M4**.

Total: 104–129 engineer-days (matches the authoritative 103–128 estimate; +1 d reconciliation delta absorbed). One engineer: 6.5 months; two: 3.5–4 months.

## 14. Recommended First Implementation Work Package

**WP-01 + WP-02 executed as the vertical foundation slice** (the prompt's "recommended first package" maps to repo bootstrap + database foundation + first end-to-end collection proof):

- **Exact scope**: monorepo bootstrap (layout, Makefile, compose, lint/depguard, CI skeleton, logging); full schema (18 tables, triggers, partitions, seed); then a single vertical proof — one location (Johor Bahru), one provider adapter (Open-Meteo), one collection job producing ForecastCollection + ForecastSnapshots with raw-payload policy applied, structured logs, integration tests, one internal inspection endpoint (admin collections query) — validating provider integration, data model, idempotency, DB design, and developer experience together.
- **Files/modules**: per repository structure doc §1 (cmd/, internal/platform, internal/collection skeleton, adapters/forecastproviders/openmeteo, adapters/persistence, migrations/).
- **Acceptance criteria**: `make dev-up` green; CI green; hourly collection runs unattended 48 h in dev; double-fire test proves zero duplicate snapshots; payload checksums verify; schema + triggers pass integration suite; deploy to scratch VPS works.
- **Tests**: migration up/down; trigger raises; dedup ON CONFLICT; adapter contract fixtures (happy/edge/drift); collection idempotency; smoke deploy.
- **Demo procedure**: run collection → show collection row + 168 snapshots + payload file + checksum via inspection endpoint + logs.
- **Risks**: partitioned-PK composite FK subtlety (spiked inside WP-02); Open-Meteo fixture capture (manual, scripted).
- **Estimated effort**: 5–7 days (WP-01: 2–3 + WP-02: 3–4).
- **Explicit non-goals**: no dashboard, no ranking system, no second provider, no Kubernetes/NATS/Temporal/Redis, no staging, no production launch.

## 15. Open Questions

**Blocking repository bootstrap or WP-01/WP-02: none.**

Non-blocking, resolved as assumptions (recorded in decision log):
- A-1: Chart library — implementation choice within 200 KB budget + capability list; decided during WP-20, not architecture.
- A-2: Trigger endpoint sync vs 202 — implementation choice; contract fields identical (reconciliation A2).
- A-3: Backup status file format — JSON convention specified (backup doc §3).
- A-4/A-5: D-05 ToS and D-06 spike — pre-existing product gates, unchanged, do not block architecture or the first package.

## 16. Architecture Quality Score

| Dimension | Score | Note |
|-----------|-------|------|
| Product alignment | 9 | Every screen/state/metric maps to a specified capability; deferred items cost zero now |
| Simplicity | 9 | One binary, one DB, no broker/queue/cache-tier; 4 deployment units |
| Data correctness | 9 | Immutable lineage end-to-end; deterministic matching; versioned reproducible metrics |
| Scalability | 8 | Vertical ceiling by design; every promotion measured + seam-prepared; not 10 because single-instance is a real (accepted) limit |
| Maintainability | 9 | Enforced module boundaries; contract-first; docs-with-code; < 2 d onboarding target |
| Security | 8 | Managed auth, structural secret exclusion, full threat model; not 10 because browser-side token exposure and single-operator key custody are inherent residuals |
| Reliability | 8 | Honest 99.5% with rehearsed recovery; not higher because single VPS (accepted, budgeted, mitigated) |
| Operability | 9 | One engineer can deploy, monitor, and recover: runbooks, drills, admin self-service, managed DB |
| Testability | 9 | Property-tested formulas, fixture-driven adapters, matrix-driven authz tests, quality gates as contract |
| Cost efficiency | 10 | ~$45/mo expected; 50%+ headroom under target; cost ceiling is provider tiers, not infra |

All scores ≥ 8; the two 8s are explained above as accepted, documented tradeoffs (not defects).

## 17. Final Decisions

### Architecture
**APPROVED.** The architecture satisfies all authoritative inputs, stays within every Phase 0 constraint, resolves all reconciliation conditions, and is operable by 1–2 engineers with measured headroom and prepared evolution paths.

### Repository bootstrap
**GO.** No blocking open questions; WP-01 scope, structure, and acceptance are fully specified; CI/security gates defined.

### Recommended first implementation work package
**GO.** WP-01+WP-02 vertical foundation slice validates the six required assumptions (provider integration, data model, idempotency, DB design, developer experience, CI gates) with bounded effort (5–7 d) and explicit non-goals.

### Full MVP implementation
**CONDITIONAL GO.** Conditions: (1) work packages proceed in the specified sequence with quality gates binding each exit; (2) D-05 (OpenWeather ToS) closed before public launch of attributed provider displays — engineering may proceed, publication gated; (3) D-06 (observation quality spike) closed before ranking publication at launch; (4) estimate governance active (weekly actuals, 20% deviation trigger); (5) any material architecture deviation during implementation returns as an ADR, not a silent drift.

A GO for repository bootstrap does not authorize simultaneous start of all packages — the sequence in §13 is binding.

---

*Deliverable index: architecture 01–10 · data 02–07 · workflows 01–06 · api 04–08 · operations 02–07 · security 02–05 · testing 02–05 · delivery 01–05 · planning 04–05 · risk 02 · ADR-013..032 · this report + decision log.*
