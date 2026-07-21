# ADR-009: Ownership Model — Single-Operator MVP, Workspace-Ready Schema

**Status**: Accepted (Phase 0 Amendment)
**Date**: 2026-07-22

## Context
Blocker 5: Phase 0 deferred multi-tenancy (open question Q7) while the domain model had
zero tenant concepts — guaranteeing an invasive retrofit. The amendment required
evaluating single-user vs. personal workspaces vs. organization workspaces, deciding
where `workspace_id` belongs, and documenting the denormalization tradeoff.

## Options considered
1. **Option A — Single-user MVP**: one operator, global locations; no ownership columns.
2. **Option B — Personal workspaces**: every user owns a private workspace.
3. **Option C — Organization workspaces**: orgs + RBAC.
4. **Hybrid A+B**: behave as A at MVP; implement B's schema now (workspace_id on
   ownership-bearing mutable entities only).

## Decision
Option 4. A `system` workspace is seeded at bootstrap. `workspace_id` (NOT NULL,
default system) on: `locations`, `provider_configurations`, `api_keys`, and reserved
for future `alert_rules`, `reports`. **Not** on immutable pipeline rows
(`forecast_collections`, `forecast_snapshots`, `observations`, `matched_evaluations`,
`accuracy_metrics`, `provider_rankings`, `audit_events`) — ownership derives via join
to the owning parent (location/collection).

## Rationale
- MVP behavior stays dead simple (one workspace), while the costly part of Option B —
  schema shape — is done once, additively.
- Denormalization tradeoff (documented per mandate): putting workspace_id on
  high-volume immutable rows would add write amplification and drift risk for zero MVP
  benefit; one indexed join derives ownership. If future workloads need workspace-
  scoped partition pruning on child tables, backfill is a documented three-step online
  migration (add nullable → backfill → set NOT NULL).
- Level 3 Option C builds on B: workspaces gain an `organization_id` parent and RLS
  policies keyed on `current_setting('app.workspace_id')`.

## Consequences
- (+) No invasive migration at Level 3; authorization queries are join + (later) RLS.
- (+) Immutability and partitioning of pipeline tables stay clean.
- (−) Every ownership check costs one join until RLS (negligible at one workspace).
- (−) Must resist adding workspace_id to child tables ad hoc (this ADR is the gate).

## Migration trigger
Enable personal workspaces (B) when: user-created locations are allowed (post-MVP
feature). Enable organizations (C) at commercial beta with its own ADR covering roles,
invites, and RLS enforcement.

## Review date
At Level 3 gate.
