# ADR-005: In-Process Go Scheduler with DB Slot Claims

**Status**: Accepted (Phase 0 Amendment)
**Date**: 2026-07-22

## Context
Phase 0 left scheduling undecided between Temporal and Kubernetes CronJobs (open
question Q2), both heavy for the approved architecture. The amendment directed a
simple database-backed or Go scheduler and no Temporal at MVP.

## Options considered
1. Temporal — durable workflows, large operational footprint (server, UI, own DB).
2. Kubernetes CronJobs — requires K8s (excluded).
3. pg_cron — DB-extension scheduling; couples job logic to SQL; weaker observability.
4. **In-process Go scheduler** (time.Ticker + cron parsing) with **DB-backed slot
   claims** (`SELECT … FOR UPDATE SKIP LOCKED` on `collection_schedules` slots) and a
   `schedule_runs` history table.

## Decision
Option 4.

## Rationale
- Hourly collection for ≤ 10 locations × 2 providers + observations is a few dozen
  jobs/day — a workflow engine is two orders of magnitude overkill.
- Slot claims make the scheduler safe under restarts and future multi-instance
  deployment (no double collection) without external coordination.
- Run history in the DB gives the admin UI (S-13) and retry/replay features for free.
- Failure handling (backoff, circuit breaker) is ordinary Go code, testable in-process.

## Consequences
- (+) Zero new infrastructure; scheduling logic is unit-testable.
- (+) Crash mid-slot: claim expires (lease timeout) and the next instance/cycle
  reclaims — idempotent collection makes retries safe.
- (−) No workflow UI/durability beyond our own run table (acceptable at this scale).
- (−) Scheduler lives in the monolith: a hung job could affect the process (mitigation:
  per-job context timeouts + watchdog metric `scheduler_missed_slots`).

## Migration trigger
Move to Temporal when: jobs need coordinated multi-step recovery/compensation, OR
scheduler miss rate > 1% from process crashes, OR job types multiply beyond
collection/observation/engine (e.g., webhooks, billing retries).

## Review date
2027-01-22 or when any trigger metric appears.
