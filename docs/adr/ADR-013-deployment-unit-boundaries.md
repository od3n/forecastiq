# ADR-013: Deployment-Unit Boundaries — One Binary, One Static Dashboard

**Status**: Accepted (Phase 1)
**Date**: 2026-07-22

## Context
ADR-001 chose the modular monolith; ADR-007 chose VPS + Caddy. Phase 1 must fix the exact deployment units and whether API and worker are separable.

## Options considered
1. Single binary, no separation ever — simplest, but blocks the documented worker-split promotion.
2. **Single binary with `--mode=api|worker|all` flag; one systemd unit at MVP running `all`; static Next.js export on CDN.**
3. Two binaries from day one (api + worker) — doubles deploy surface for zero MVP benefit.

## Decision
Option 2. Four deployment units total: forecastiq binary, dashboard static build, managed PostgreSQL, Caddy.

## Rationale
- One process = one pool, one log stream, one restart path; scheduler goroutine needs no IPC.
- The mode flag costs ~zero now and makes the worker-split promotion (constraints §4) a deploy change, not a code change.
- Static dashboard export removes the Node server from the runtime entirely (CDN serves; no SSR dependency).

## Consequences
- (+) Minimal ops surface; graceful shutdown drains scheduler + HTTP together (30 s).
- (+) Promotion path preserved and tested by the flag's existence.
- (−) A hung batch can contend with API for CPU until split (mitigated: job timeouts + watchdog; promotion trigger measured).

## Risks
CPU contention at MVP volume is theoretical (batch < 10 min/30 min cycle; API < 20 qps effective) — monitored via PT-1/PT-4 baselines.

## Migration trigger
Per constraints §4: batch > 10 min or API p95 correlation with batch windows.

## Review date
At Level 1 exit (first performance baseline).
