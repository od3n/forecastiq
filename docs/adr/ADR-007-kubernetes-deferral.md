# ADR-007: Kubernetes Deferral — Single VPS + Caddy for MVP

**Status**: Accepted (Phase 0 Amendment)
**Date**: 2026-07-22

## Context
Phase 0 NFRs required Kubernetes (multi-AZ, cert-manager, K8s Secrets, service
accounts) while constraints said 1–2 engineers and < $500/mo. The ARB called the
combination infeasible (Conflict #3) and the DevOps/SRE dissent recommended NO-GO on
this basis.

## Options considered
1. Managed Kubernetes (EKS/GKE) — still substantial ops, cost, and conceptual surface.
2. Platform-as-a-Service containers (Fly.io/Render) — simpler, but multi-component
   cost climbs and debugging is more opaque.
3. **Single VPS + Caddy (auto-TLS) + managed PostgreSQL + CDN for the dashboard** —
   one machine, one process, one DB; target $50–150/mo.

## Decision
Option 3, with deployment via GitHub Actions (build → migrate → deploy artifact;
rollback = redeploy previous artifact < 5 min).

## Rationale
- The workload (one binary, dozens of jobs/day, tens of concurrent users) needs no
  orchestration. K8s solves problems the MVP does not have.
- Caddy provides automatic TLS 1.3 certificates with zero ceremony (replaces
  cert-manager).
- Managed DB removes the largest operational risk (backups, PITR, patching) while
  keeping a separate failure domain from the VPS.
- Directly implements the dissenting opinion's "simplest thing that works" mandate.

## Consequences
- (+) ~$50/mo vs. hundreds; ops surface a single engineer can hold in their head.
- (+) Fast, comprehensible deploys and rollbacks.
- (−) Single-VPS availability ceiling → NFR-A01 honestly set to 99.5%.
- (−) Rebuild-from-scratch procedure must be excellent (runbook + monthly restore
  test) — accepted, documented (NFR-A05, DR table).

## Migration trigger
Adopt K8s when: ≥ 3 independently scaled services AND ≥ 4 engineers, OR availability
commitment > 99.5% (customer SLAs at Level 3), OR multi-AZ becomes a sales
requirement. Intermediate step: second instance + load balancer (no K8s) if a single
extra nine is needed first.

## Review date
At Level 3 gate.
