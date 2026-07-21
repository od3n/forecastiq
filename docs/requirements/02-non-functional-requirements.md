# ForecastIQ — Non-Functional Requirements (Revised)

**Version**: 2.0 (Phase 0 Amendment)
**Status**: Authoritative
**Supersedes**: `docs/phase-0-business-analysis/05-non-functional-requirements.md`

Targets are right-sized to the approved architecture (single VPS + managed Postgres).
Each row notes the change vs. Phase 0 where applicable.

---

## 1. Performance

| ID | Requirement | Target | Measurement | Δ vs Phase 0 |
|----|-------------|--------|-------------|--------------|
| NFR-P01 | API p50 | < 50 ms | `/metrics` histogram | unchanged |
| NFR-P02 | API p95 | < 200 ms | histogram | unchanged |
| NFR-P03 | API p99 | < 500 ms | histogram | unchanged |
| NFR-P04 | Dashboard initial meaningful paint | < 2 s | Lighthouse/RUM | unchanged |
| NFR-P05 | Sustained API throughput | ≥ 100 req/s | load test | **reduced from 1,000** — MVP reality; promotion trigger defined |
| NFR-P06 | Comparison batch | < 10 min for 100K pairs | job metrics | unchanged |
| NFR-P07 | Collection cycle completion | < 5 min for all providers/locations | job metrics | unchanged |
| NFR-P08 | DB query p95 | < 100 ms | pg_stat_statements | unchanged |

## 2. Availability & Reliability

| ID | Requirement | Target | Δ vs Phase 0 |
|----|-------------|--------|--------------|
| NFR-A01 | API availability | **99.5 %** (single-VPS honest target; 99.9 % is Level 3 with promoted infra) | reduced, justified |
| NFR-A02 | Dashboard availability | 99.5 % (CDN-served static) | aligned |
| NFR-A03 | Collection pipeline success rate | ≥ 99 % of scheduled slots completed per month | clarified |
| NFR-A04 | RPO | < 1 h (managed DB PITR + hourly WAL) | unchanged |
| NFR-A05 | RTO | < 4 h (redeploy-to-new-VPS runbook + restore) | unchanged |
| NFR-A06 | Zero loss of stored forecasts/observations | 100 % (immutability + DB durability) | unchanged |
| NFR-A07 | Graceful degradation | Stale-cache serving with explicit staleness labels during transient DB issues; provider outages isolated per provider (circuit breaker) | reworded: no Redis requirement; honesty-first degradation |
| NFR-A08 | No SPOF | **Deferred** (single VPS accepted for MVP; managed DB is a separate failure domain; mitigation = fast rebuild runbook) | removed K8s replica requirement |

## 3. Scalability (design headroom, not MVP load)

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-S01 | Storage design headroom | Schema/partitioning validated to 100M snapshot rows (load test at 2× MVP volume; extrapolation documented) |
| NFR-S02 | Collection scaling | Parallel per provider within the process; rate-limit aware |
| NFR-S03 | Concurrent dashboard users | 100 concurrent (MVP); CDN caching of static assets |
| NFR-S04 | Scale-out path | Documented promotions: read replica → separate analytics worker → second app instance (+Redis) → services split (K8s gate) |

## 4. Security

| ID | Requirement | Standard | Implementation (MVP) |
|----|-------------|----------|----------------------|
| NFR-SEC01 | TLS in transit | TLS 1.3 | Caddy + Let's Encrypt automation |
| NFR-SEC02 | Secrets at rest | encrypted | Managed DB encryption + env-injected secrets; no secrets in repo |
| NFR-SEC03 | API key hashing | argon2/bcrypt | never plaintext |
| NFR-SEC04 | Token verification | RS256 via JWKS | Supabase JWKS; no local signing keys |
| NFR-SEC05 | Input validation | OWASP | middleware validation on all endpoints |
| NFR-SEC06 | SQL injection | parameterized queries | query builder; no string SQL |
| NFR-SEC07 | Rate limiting | per-key + per-IP on auth-adjacent | in-process limiter (Redis promotion criteria defined) |
| NFR-SEC08 | CORS | allowlist | dashboard origin + localhost dev |
| NFR-SEC09 | Security headers | OWASP set | Caddy + app middleware |
| NFR-SEC10 | Dependency scanning | no critical CVEs | CI (govulncheck + Trivy on image) |
| NFR-SEC11 | Audit logging | all auth + admin actions | audit_events table |
| NFR-SEC12 | Least privilege DB | single app credential, no superuser in app | managed DB scoped role |
| NFR-SEC13 | Secret rotation | ≤ 90 days | runbook: provider keys, DB creds |
| NFR-SEC14 | OWASP Top 10 | annual checklist review | unchanged |
| NFR-SEC15 | **Password policy** (new) | min 12 chars (or 8 + breach-list check via managed auth defaults), no composition theater | Supabase Auth config |
| NFR-SEC16 | **Concurrent session policy** (new) | allowed; refresh rotation detects token theft (reuse → family revocation) | managed auth behavior + app audit |

## 5. Observability (simplified stack, same discipline)

| ID | Requirement | Implementation |
|----|-------------|----------------|
| NFR-OBS01 | Structured JSON logging (timestamp RFC3339, level, request_id, service, message, fields) | slog → hosted log service |
| NFR-OBS02 | Request correlation | `X-Request-Id` propagated to logs (replaces distributed tracing for MVP; tracing is a promotion) |
| NFR-OBS03 | RED metrics per module | Prometheus `/metrics` + hosted Grafana free tier |
| NFR-OBS04 | Health endpoints | `/healthz`, `/readyz` |
| NFR-OBS05 | Alerting | log/metric thresholds → email/webhook (hosted) for: collection stale, circuit open, schema drift, disk > 80 %, cert expiry < 14 d, backup failure |
| NFR-OBS06 | Ops dashboard | Grafana: collection success, engine lag, API RED, DB pool, disk |
| NFR-OBS07 | SLO tracking | availability + latency SLOs tracked monthly |

## 6. Data Management

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-D01 | Snapshot retention | 2 years (partition drop) |
| NFR-D02 | Metrics retention | indefinite |
| NFR-D03 | Observation retention | 5 years |
| NFR-D04 | Audit retention | 1 year |
| NFR-D05 | Raw payload retention | 90 days (ADR-011) |
| NFR-D06 | Backups | managed PITR + nightly pg_dump + weekly offsite; **monthly automated restore test** (result visible in admin) |
| NFR-D07 | Encryption at rest | managed DB disk encryption |
| NFR-D08 | GDPR | account export + deletion (AUTH-09); weather data is not personal data (documented position) |
| NFR-D09 | Timezones | UTC storage; display per BR-TZ |

## 7. Maintainability

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-M01 | Unit coverage | ≥ 80 % on analysis/domain packages; 100 % of methodology formulas covered incl. property tests |
| NFR-M02 | Integration coverage | collection→match→metric→rank golden path; adapter contract tests against recorded fixtures |
| NFR-M03 | Lint | zero golangci-lint warnings |
| NFR-M04 | Docs | OpenAPI spec, ADRs, runbooks, methodology page |
| NFR-M05 | Deploy frequency | ≥ 1/week |
| NFR-M06 | Time to deploy | < 30 min |
| NFR-M07 | Rollback | < 5 min (redeploy previous artifact) |
| NFR-M08 | Onboarding | < 2 days to first PR (docs + simple arch) |
| NFR-M09 | Contract testing | OpenAPI spec check in CI (consumer-driven Pact deferred to Level 3) |

## 8. Compatibility

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-C01 | API versioning | URL `/api/v1/` + `Sunset`/`Deprecation` headers |
| NFR-C02 | Breaking changes | new major, ≥ 6-month deprecation |
| NFR-C03 | Browsers | last 2 versions of Chrome/Firefox/Safari/Edge |

## 9. Compliance

| ID | Requirement | Standard |
|----|-------------|----------|
| NFR-CMP01 | Provider attribution | per ToS; BR-ATTR-01 |
| NFR-CMP02 | ToS validation gate | D-05 before public launch |
| NFR-CMP03 | Privacy policy + terms | published at launch |
| NFR-CMP04 | Accessibility | WCAG 2.1 AA target with concrete checks (contrast, focus order, alt text, keyboard) |

## 10. Disaster Recovery (revised for single VPS)

| Scenario | RPO | RTO | Strategy |
|----------|-----|-----|----------|
| VPS failure | < 1 h | < 4 h | New VPS + deploy pipeline + managed DB reconnect + payload volume re-sync (90 d payload loss acceptable per retention policy; DB intact) |
| DB corruption | < 1 h | < 2 h | PITR from managed WAL |
| Accidental deletion | < 1 h | < 1 h | Immutability triggers prevent pipeline-table deletes; backups for the rest |
| Provider API outage | — | — | Circuit breaker; freshness states; other providers unaffected |
| Payload volume corruption | — | < 24 h | Normalized data unaffected; payloads rebuild only via future collections; checksum detects per-file |
