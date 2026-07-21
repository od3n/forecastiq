# ForecastIQ — Non-Functional Requirements

**Version**: 1.0  
**Status**: Draft  

---

## 1. Performance

| ID | Requirement | Target | Measurement |
|----|-------------|--------|-------------|
| NFR-P01 | API response time (p50) | < 50ms | Prometheus histogram |
| NFR-P02 | API response time (p95) | < 200ms | Prometheus histogram |
| NFR-P03 | API response time (p99) | < 500ms | Prometheus histogram |
| NFR-P04 | Dashboard initial load | < 2s | Lighthouse / RUM |
| NFR-P05 | Dashboard interactive | < 3s | Lighthouse / RUM |
| NFR-P06 | Sustained API throughput | ≥ 1,000 req/s | Load test |
| NFR-P07 | Burst API throughput | ≥ 5,000 req/s (10s) | Load test |
| NFR-P08 | Comparison engine batch | < 10 min for 100K pairs | Job metrics |
| NFR-P09 | Collection cycle completion | < 5 min for all providers | Job metrics |
| NFR-P10 | Database query time (p95) | < 100ms | pg_stat_statements |

---

## 2. Availability & Reliability

| ID | Requirement | Target | Measurement |
|----|-------------|--------|-------------|
| NFR-A01 | API availability | 99.9% (8.7h downtime/year) | Uptime monitoring |
| NFR-A02 | Dashboard availability | 99.5% | Uptime monitoring |
| NFR-A03 | Collection pipeline availability | 99.0% | Job success rate |
| NFR-A04 | RPO (Recovery Point Objective) | < 1 hour | Backup verification |
| NFR-A05 | RTO (Recovery Time Objective) | < 4 hours | DR drill |
| NFR-A06 | Zero data loss for stored forecasts | 100% | Immutability + replication |
| NFR-A07 | Graceful degradation | API serves cached data during DB outage | Chaos test |
| NFR-A08 | No single point of failure (API layer) | ≥ 2 replicas | K8s deployment |

---

## 3. Scalability

| ID | Requirement | Target | Approach |
|----|-------------|--------|----------|
| NFR-S01 | Horizontal API scaling | Linear with replicas | Stateless services |
| NFR-S02 | Database read scaling | Read replicas for analytics | TimescaleDB replicas |
| NFR-S03 | Collection scaling | Parallel per provider/location | Worker pools |
| NFR-S04 | Storage scaling | Handle 100M forecast records | Partitioning + retention |
| NFR-S05 | Concurrent users | 10,000 dashboard sessions | CDN + caching |

---

## 4. Security

| ID | Requirement | Standard | Implementation |
|----|-------------|----------|---------------|
| NFR-SEC01 | All traffic encrypted in transit | TLS 1.3 | Ingress/cert-manager |
| NFR-SEC02 | Secrets encrypted at rest | AES-256 | K8s Secrets + KMS |
| NFR-SEC03 | API key hashing | bcrypt/argon2 | Never store plaintext |
| NFR-SEC04 | JWT signing | RS256 (asymmetric) | Key rotation support |
| NFR-SEC05 | Input validation | OWASP | Middleware validation |
| NFR-SEC06 | SQL injection prevention | Parameterized queries | ORM/query builder |
| NFR-SEC07 | Rate limiting | Per-key, per-IP | Redis-backed limiter |
| NFR-SEC08 | CORS policy | Whitelist origins | Middleware |
| NFR-SEC09 | Security headers | OWASP recommended | Middleware |
| NFR-SEC10 | Dependency scanning | No critical CVEs | CI pipeline (Trivy) |
| NFR-SEC11 | Audit logging | All auth + admin actions | Structured audit log |
| NFR-SEC12 | Least privilege DB access | Per-service credentials | K8s service accounts |
| NFR-SEC13 | Secret rotation | ≤ 90 days | Automated rotation |
| NFR-SEC14 | OWASP Top 10 compliance | Annual review | Security checklist |

---

## 5. Observability

| ID | Requirement | Tool | Standard |
|----|-------------|------|----------|
| NFR-OBS01 | Structured JSON logging | Loki + Promtail | All services |
| NFR-OBS02 | Distributed tracing | OpenTelemetry + Jaeger | All HTTP + DB calls |
| NFR-OBS03 | Metrics (RED method) | Prometheus + Grafana | Rate, Errors, Duration |
| NFR-OBS04 | Health endpoints | `/health`, `/ready` | All services |
| NFR-OBS05 | Alerting rules | Grafana Alerting | Error rate, latency, saturation |
| NFR-OBS06 | Log correlation | Trace ID in logs | OpenTelemetry context |
| NFR-OBS07 | Dashboard (ops) | Grafana | Pre-built service dashboards |
| NFR-OBS08 | SLO tracking | Grafana SLO | Availability, latency |

---

## 6. Data Management

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-D01 | Raw forecast retention | 2 years (then archive to S3) |
| NFR-D02 | Aggregated metrics retention | Indefinite |
| NFR-D03 | Observation retention | 5 years |
| NFR-D04 | Audit log retention | 1 year |
| NFR-D05 | Backup frequency | Daily full, hourly WAL archive |
| NFR-D06 | Backup verification | Weekly automated restore test |
| NFR-D07 | Data encryption at rest | AES-256 (disk-level) |
| NFR-D08 | GDPR compliance | User data export + deletion |
| NFR-D09 | Timezone handling | All stored UTC, display local |

---

## 7. Maintainability

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-M01 | Code coverage (unit) | ≥ 80% |
| NFR-M02 | Code coverage (integration) | Key paths covered |
| NFR-M03 | Lint compliance | Zero warnings (golangci-lint) |
| NFR-M04 | Documentation | OpenAPI spec, ADRs, runbooks |
| NFR-M05 | Deployment frequency | ≥ 1/week |
| NFR-M06 | Mean time to deploy | < 30 min |
| NFR-M07 | Rollback time | < 5 min |
| NFR-M08 | Onboarding time (new dev) | < 2 days to first PR |

---

## 8. Compatibility

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-C01 | API versioning | URL-based (`/api/v1/`) |
| NFR-C02 | Breaking changes | New major version, 6-month deprecation |
| NFR-C03 | Browser support | Chrome, Firefox, Safari, Edge (last 2 versions) |
| NFR-C04 | API client support | Any HTTP client (JSON) |

---

## 9. Compliance

| ID | Requirement | Standard |
|----|-------------|----------|
| NFR-CMP01 | Provider attribution | Per provider ToS |
| NFR-CMP02 | Data licensing | Clear terms of service |
| NFR-CMP03 | Privacy policy | GDPR/CCPA compliant |
| NFR-CMP04 | Accessibility (dashboard) | WCAG 2.1 AA |

---

## 10. Disaster Recovery

| Scenario | RPO | RTO | Strategy |
|----------|-----|-----|----------|
| Single AZ failure | 0 | < 5 min | Multi-AZ K8s |
| Region failure | < 1h | < 4h | Cross-region backup + restore |
| Database corruption | < 1h | < 2h | PITR from WAL archive |
| Accidental deletion | < 1h | < 1h | Soft delete + backup |
| Provider API outage | N/A | N/A | Circuit breaker, cached data |
