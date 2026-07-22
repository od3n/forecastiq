# ADR-026: Managed Hosting Platform — Hetzner VPS + Neon + Cloudflare

**Status**: Accepted (Phase 1) — specifies ADR-007
**Date**: 2026-07-22

## Context
ADR-007 chose "single VPS + Caddy + managed PostgreSQL + CDN" at $50–150/mo. Phase 1 selects concrete vendors.

## Options considered
| Option | Monthly (expected) | Verdict |
|--------|--------------------|---------|
| **Hetzner CX32 + Neon (managed PG16, PITR) + Cloudflare (DNS/Pages) + Supabase Auth** | ~$42–47 | **Selected** |
| Fly.io + Fly Postgres | ~$60–90 | Alternative (documented fallback) |
| Render (web service + Postgres) | ~$70–100 | Rejected (cost, less control) |
| AWS App Runner + RDS | ~$80–150 | Rejected (IAM/VPC overhead for one binary; portfolio opacity) |
| DigitalOcean equivalent | ~$55–65 | Acceptable substitute if Hetzner region/availability is an issue |

## Decision
Hetzner CX32-class VPS (4 vCPU/8 GB) + 50 GB encrypted volume; Neon paid tier (PITR, 3 GB+); Cloudflare DNS + Pages (dashboard); Supabase Auth (ADR-008); Grafana Cloud free tier (observability); Backblaze B2 (offsite dumps).

## Rationale
- Best compute-per-dollar in the constraint envelope → headroom for the entire MVP lifecycle without a platform change.
- Neon: PITR included at entry tier, branching for migration dry-runs, PostgreSQL 16 without extensions (matches ADR-004 portability).
- Cloudflare Pages: zero-cost zero-ops dashboard hosting with PR previews; DNS API enables Terraform-managed failover.
- Every vendor has a free/cheap tier exit: switching DB vendor = pg_dump/restore + DSN change (no extension dependencies by design).

## Consequences
- (+) ~$42–47/mo expected — 50%+ under target ceiling; cost headroom documented (data doc 06 §5).
- (+) Separate failure domains: DB (managed) / VPS / CDN / auth.
- (−) Hetzner has no managed K8s-style conveniences — irrelevant (excluded by ADR-007).
- (−) Single region (EU) — acceptable for portfolio MVP; multi-region is a Level 3 concern.

## Risks
Vendor availability in chosen region → DigitalOcean substitute documented (same architecture, ~+$10/mo).

## Migration trigger
Availability commitment > 99.5% (customer SLAs) → second instance + LB, then multi-AZ path (constraints §4 / ADR-007 triggers).

## Review date
At Level 3 gate; on any vendor pricing change > 20%.
