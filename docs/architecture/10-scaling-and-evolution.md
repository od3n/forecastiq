# ForecastIQ — Scaling and Evolution Plan (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: Constraints §4 (promotion criteria — this document operationalizes them); NFR-S01..S04; ADR-001/004/005/006/007

**Principle:** every promotion is triggered by **measurement in production**, never anticipation. Each is a small ADR-supervised migration with a defined path. Premature adoption is itself a risk (constraints §3 rationale).

---

## 1. Promotion Register

### 1.1 Redis (cache + rate-limit coordination)

| Attribute | Specification |
|-----------|---------------|
| Measurable trigger | p95 API latency > 150 ms dominated by repeated ranking/metric DB reads (pg_stat_statements evidence), OR second app instance deployed requiring shared rate-limit state |
| Expected benefit | Cross-restart caching; shared rate limiting; session-state sharing if ever needed |
| Migration path | Cache port already abstracted (`Cache` interface in api module) → swap LRU implementation for Redis client; rate limiter moves from in-process token bucket to Redis INCR with expiry. Zero domain changes. |
| Risk | Another service to operate; cache invalidation semantics; new failure mode (mitigated: cache-aside with DB fallback) |
| Decision owner | Operator (eng) |

### 1.2 Separate worker deployment

| Attribute | Specification |
|-----------|---------------|
| Measurable trigger | Comparison batch > 10 min (NFR-P06 breach) OR collection jobs measurably degrade API p95 (correlated CPU saturation in Grafana) |
| Expected benefit | Independent scaling of batch CPU; API isolation |
| Migration path | Binary already supports `--mode=api|worker|all` flag design (Phase 1 implementation includes the flag, unused); deploy second systemd unit with mode=worker; scheduler slot claims (SKIP LOCKED) already multi-instance-safe |
| Risk | Deployment/pipeline duplication (modest); log/metric label additions |
| Decision owner | Operator |

### 1.3 Read replica

| Attribute | Specification |
|-----------|---------------|
| Measurable trigger | Analytics/dashboard queries degrade write path: p95 insert latency > 50 ms correlated with dashboard load, sustained over 1 week |
| Expected benefit | Read/write isolation |
| Migration path | Managed DB replica (vendor feature); analysis module read DSN → replica; eventual consistency acceptable for metric reads (batch-computed anyway) |
| Risk | Replication lag semantics (freshness labels must account); cost |
| Decision owner | Operator |

### 1.4 NATS JetStream (event bus)

| Attribute | Specification |
|-----------|---------------|
| Measurable trigger | Comparison engine lag > 15 min behind collection at sustained volume, OR a second consumer of collection events exists (webhooks, analytics export), OR modules extracted to separate processes |
| Expected benefit | Decoupled pipeline, replayable events, multiple consumers |
| Migration path | In-process event seam (ADR-006) already uses versioned names/payloads → publish at the same seam to JetStream; consumers become JetStream consumers (ack explicit; streams FORECASTS/OBSERVATIONS, 7 d retention, limits policy) |
| Risk | Schema governance; ordering subtleties; broker ops |
| Decision owner | Operator + architecture review |

### 1.5 Temporal (durable workflows)

| Attribute | Specification |
|-----------|---------------|
| Measurable trigger | Jobs require coordinated multi-step recovery/compensation beyond simple retries, OR scheduler miss rate > 1% due to process crashes mid-job, OR job types multiply beyond collection/observation/engine |
| Expected benefit | Durable multi-step workflows; visibility UI; complex retry state |
| Migration path | Scheduler module boundary isolated (ADR-005); replace ticker+claims with Temporal workflows per job type; DB slot tables become Temporal state |
| Risk | Large dependency (server + UI + own DB); conceptual overhead for 1–2 engineers |
| Decision owner | Architecture review (material change → ADR) |

### 1.6 Kubernetes

| Attribute | Specification |
|-----------|---------------|
| Measurable trigger | ≥ 3 independently scaled services AND ≥ 4 engineers, OR availability commitment > 99.5% (customer SLAs), OR multi-AZ becomes a sales requirement |
| Expected benefit | Multi-service orchestration; rolling deploys; resource isolation |
| Migration path | Containerize modules as separate services along existing module boundaries (ADR-001 seams); Helm charts; ingress + cert-manager. Intermediate step: second VPS instance + load balancer (no K8s) if one extra nine is needed first. |
| Risk | Highest ops burden of any promotion; a 1–2 person team cannot operate it safely |
| Decision owner | Architecture review + business gate (Level 3) |

### 1.7 Object storage (S3-compatible)

| Attribute | Specification |
|-----------|---------------|
| Measurable trigger | Payload volume > 50 GB, OR retention requirement > 90 d (compliance/customer), OR backup size dominated by payloads |
| Expected benefit | Cheap durable retention; lifecycle policies |
| Migration path | `raw_payload_object_key` is scheme-prefixed (ADR-011) → add `s3://` writer behind the same PayloadStore interface; lifecycle rules; old volume files age out naturally |
| Risk | Another credential + dependency; egress costs |
| Decision owner | Operator |

### 1.8 TimescaleDB

| Attribute | Specification |
|-----------|---------------|
| Measurable trigger | Sustained p95 query > 200 ms on partitioned+indexed tables at real load, OR retention ops become manual burden, OR storage > 100 GB makes compression economic (ADR-004) |
| Expected benefit | Hypertables, compression, continuous aggregates, retention policies |
| Migration path | Extension install + hypertable conversion on forecast_snapshots/observations; continuous aggregates only with measured aggregation cost |
| Risk | Extension lock-in; narrows managed-DB options |
| Decision owner | Architecture review with load-test evidence |

### 1.9 Staging environment

| Attribute | Specification |
|-----------|---------------|
| Measurable trigger | Second engineer onboarded, OR customer-facing SLAs, OR migration complexity exceeds dry-run confidence |
| Migration path | Clone production topology at reduced size; same pipeline with staging stage |
| Risk | Cost + maintenance for limited MVP value |
| Decision owner | Operator |

## 2. Seams That Make Promotions Cheap (built in Phase 1)

| Seam | Implementation | Enables |
|------|----------------|---------|
| Cache interface | `Cache` port in api module (LRU impl at MVP) | Redis swap |
| PayloadStore interface | Filesystem impl; scheme-prefixed keys | S3 swap |
| Event seam | Versioned in-process bus; stable names/payloads | NATS swap |
| Scheduler mode flag | `--mode=api|worker|all` | Worker split |
| SKIP LOCKED claims | Multi-instance-safe from day one | Horizontal scale |
| Module boundaries | Interface-only cross-module access | Service extraction |
| workspace_id columns | On ownership-bearing parents | RLS / multi-tenancy |
| Read/write DSN split point | Single DSN now; config shape allows second | Read replica |

## 3. Anti-Patterns Guard

- No promotion without a Grafana screenshot / metric query demonstrating the trigger (review checklist item).
- No "while we're here" infrastructure additions during feature work (scope rule R-04).
- Each promotion gets its own ADR referencing this register and the measured evidence.
- Promotions are estimated separately at trigger time (not pre-budgeted in the MVP estimate).

## 4. Growth Scenarios (order of adoption)

```text
MVP (now) → [latency trigger] → Redis → [batch CPU trigger] → worker split →
[read pressure] → read replica → [second consumer / extraction] → NATS →
[multi-service + team ≥ 4] → Kubernetes
```

Each step is independently valuable and independently reversible. The architecture never requires step N+1 for step N to work.

## 5. Cross-Reference

- Constraints promotion table (authoritative triggers): `docs/architecture/00-phase-0-architecture-constraints.md` §4
- ADRs: ADR-001, ADR-004, ADR-005, ADR-006, ADR-007, ADR-011
- Risk: R-06 (operational complexity, mitigated by design)
