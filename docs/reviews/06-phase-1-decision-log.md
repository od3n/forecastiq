# ForecastIQ — Phase 1 Decision Log

**Version**: 1.0
**Date**: 2026-07-22
**Companion**: `docs/reviews/05-phase-1-architecture-report.md`

All material Phase 1 decisions, their status, and where the full rationale lives. Phase 0 ADRs (001–012) remain authoritative and are not re-listed except where Phase 1 extends them.

---

## 1. Architecture Decisions (ADR-recorded)

| # | Decision | Status | Instrument | Supersedes/Extends |
|---|----------|--------|-----------|--------------------|
| 1 | Modular monolith (Go, package-level module boundaries) | Accepted (Phase 0) | ADR-001 | — |
| 2 | Deployment units: one binary (+ mode flag) + static dashboard + managed PG + Caddy | Accepted | ADR-013 | Specifies ADR-001/007 |
| 3 | Standard PostgreSQL 16 over TimescaleDB | Accepted (Phase 0) | ADR-004 | — |
| 4 | In-process scheduler with DB slot claims (no Temporal/pg_cron) | Accepted (Phase 0) | ADR-005 | — |
| 5 | ForecastCollection → ForecastSnapshot lineage model | Accepted (Phase 0) | ADR-012 | — |
| 6 | Observation model: direct rows, polled with 2 h backfill window, no parent entity | Accepted | ADR-025 | Implements ADR-003; ratifies reconciliation verdict |
| 7 | Matching: deterministic exact-hour, total-order selection, append-only rematch | Accepted | ADR-014 | — |
| 8 | Evaluation: 30-min batch, in-memory pair eval, persisted immutable metrics | Accepted | ADR-015 | — |
| 9 | Rankings: stored immutable projections, atomic publication, supersede history | Accepted | ADR-016 | — |
| 10 | Authentication: Supabase Auth, JWKS verification, stateless API | Accepted (Phase 0) | ADR-008 | — |
| 11 | Authorization: middleware + object checks + repo scoping; no RLS at MVP | Accepted | ADR-017 | Extends ADR-009 path |
| 12 | Raw payloads: volume, 90 d, checksums forever | Accepted (Phase 0) | ADR-011 | — |
| 13 | Payload implementation: PayloadStore interface, scheme-prefixed keys, no file-serving route | Accepted | ADR-019 | Extends ADR-011 |
| 14 | API composition: domain endpoints + 2 screen endpoints, no BFF, ≤ 2 req/screen | Accepted | ADR-018 | Ratifies board mandate |
| 15 | Hosting: Hetzner + Neon + Cloudflare + Grafana Cloud + B2 | Accepted | ADR-026 | Specifies ADR-007 |
| 16 | Object storage (S3) deferred with trigger | Accepted | ADR-019 + constraints §4 | — |
| 17 | Redis deferred; LRU + ETag + stale-serving | Accepted | ADR-020 | Ratifies constraints §3/§4 |
| 18 | Internal events: five versioned in-process events, advisory consumers | Accepted | ADR-021 | Implements ADR-006 |
| 19 | Identifiers: UUIDv7 everywhere | Accepted | ADR-022 | Ratifies domain model §11 |
| 20 | Repository: monorepo, depguard-enforced module boundaries | Accepted | ADR-023 | — |
| 21 | Backup/DR: PITR + nightly dumps + offsite + monthly restore test; no payload backup | Accepted | ADR-024 | — |
| 22 | Transactions: short bounded txs, chunked batches, no outbox | Accepted | ADR-027 | — |
| 23 | Stale-serving: expired-LRU reads with forced staleness; mutations fail hard | Accepted | ADR-028 | Implements NFR-A07 |
| 24 | Partitioning: monthly declarative; DROP-based retention; maintenance-GUC purge exemption | Accepted | ADR-029 | Implements ADR-004 |
| 25 | Methodology registry: versioned code config, not a table | Accepted | ADR-030 | Ratifies reconciliation verdict |
| 26 | Environments: local/CI/production; no staging | Accepted | ADR-031 | — |
| 27 | Quality gates bind work-package completion | Accepted | ADR-032 | — |
| 28 | Kubernetes deferred | Accepted (Phase 0) | ADR-007 | — |
| 29 | Event bus (NATS) deferred with seam | Accepted (Phase 0) | ADR-006 | — |
| 30 | Ownership: single-operator behavior, workspace-ready schema | Accepted (Phase 0) | ADR-009 | — |

## 2. Design Decisions (document-recorded, below ADR threshold)

| # | Decision | Location |
|---|----------|----------|
| D-1 | 8 internal modules with table-ownership map | architecture/03 |
| D-2 | 24-index physical set (incl. reconciliation's additive rankings index) | data/04 |
| D-3 | 18-table schema; export_jobs + provider_circuits added per reconciliation | data/03 |
| D-4 | Immutability trigger design + maintenance GUC exemption | data/02 §5, data/03 |
| D-5 | Circuit breaker persistence in provider_circuits (restart-safe) | domain/04 reconciliation ratification; workflows/01 |
| D-6 | Retry policy: 1/2/4/8/16 s ±20% jitter, max 5; non-retryable 4xx | workflows/01 §5 |
| D-7 | Collection dedup key: (provider, location, COALESCE(model_run_time, requested_at)) | data/03, workflows/01 §7 |
| D-8 | Observation correction ε thresholds (0.1 °C, 0.05 mm, ...) | workflows/02 §4 |
| D-9 | Batch chunk sizes: snapshots ~400, pairs 1,000, metrics 500, purges 10,000 | ADR-027 table |
| D-10 | LRU: 256 entries, per-class TTL, auth-class cache keys | api/08 |
| D-11 | Envelope applicability rules (no null placeholders) | api/02 (Phase 0) + api/04 |
| D-12 | 34-endpoint catalog; screen mapping ≤ 2 requests | api/05 |
| D-13 | 17 alert rules with named procedures | operations/03 |
| D-14 | SLI set + error-budget policy | operations/02 |
| D-15 | Supersede-purge policy (2 y full history; monthly snapshots indefinite) | data/06 §4 |
| D-16 | Test data: synthetic only; JB canonical location | testing/05 |
| D-17 | Audit action registry (stable names) | security/05 |
| D-18 | Data classification: 4 tiers with handling rules | security/03 |
| D-19 | IaC split: Terraform (DNS+DB) / scripted bootstrap (VPS) / committed configs | delivery/04 |
| D-20 | Trunk-based branching; calendar tags; conventional commits | delivery/02 |
| D-21 | WP sequence + milestone gates M1–M4 | delivery/05 |
| D-22 | First package = WP-01+WP-02 vertical slice | report §14 |

## 3. Assumptions (non-blocking open questions resolved)

| # | Assumption | Owner | Revisit |
|---|-----------|-------|---------|
| A-1 | Chart library chosen in WP-20 within 200 KB + capability budget | Frontend | WP-20 start |
| A-2 | Trigger endpoint sync implementation (200 vs 202); contract identical | Backend | WP-08 |
| A-3 | Backup status file: JSON convention | Ops | WP-24 |
| A-4 | Open-Meteo Historical observation_type defaults to `reanalysis` where API doesn't expose per-variable provenance | Eng | At adapter build (WP-09) + D-06 spike |
| A-5 | D-05 (ToS) and D-06 (quality spike) are launch gates, not engineering blockers | Operator | Pre-launch |

## 4. Conflicts Arbitrated

**None.** All Phase 0 conflicts were resolved by the Amendment and Reconciliation Board. Phase 1 found no remaining inter-document contradictions; where documents overlap (e.g., constraints §4 vs. scaling doc), the Phase 0 document is normative and the Phase 1 document operationalizes it (noted inline in each).

## 5. Governance

- ADRs immutable once Accepted; changes via new ADR (supersede reference).
- Design decisions (D-*) change via PR to the owning document with review.
- This log updated at each milestone (M1–M4) with any decisions made during implementation.
