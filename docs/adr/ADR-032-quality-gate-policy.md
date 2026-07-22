# ADR-032: Quality Gate Policy — Test Gates Bind Work Package Completion

**Status**: Accepted (Phase 1)
**Date**: 2026-07-22

## Context
The prompt requires "the minimum test gate before implementation phases are considered complete." Phase 1 defines the gate and makes it a delivery contract.

## Options considered
1. Coverage target only (e.g., 80% everywhere) — measures presence, not correctness of the critical surface.
2. **Layered gate: formula coverage 100% + 11 property invariants in CI; package coverage ≥ 80% on touched packages; contract tests for touched adapters/endpoints; golden path green; lint zero; OpenAPI drift clean; no untracked skipped tests. Release gate adds reliability + performance + accessibility + worked-example reproduction.**
3. Gate-by-review-discretion — inconsistent; rejected (the amendment's quality discipline requires mechanical enforcement).

## Decision
Option 2, per `docs/testing/02-testing-strategy.md` §5.

## Rationale
- The product's credibility risk is concentrated in formulas and data lineage — the gate overweights exactly those (100% + properties + byte-exact worked example).
- Mechanical gates remove scope-pressure exceptions ("cut scope, never quality gates" — estimate §5).
- Accessibility in the gate from the first screen (reconciliation mandate) — retrofit a11y is the expensive kind.

## Consequences
- (+) Every work package exits with verifiable quality (WP acceptance criteria reference this gate).
- (+) CI is the single arbiter — no "we'll add tests later" state exists.
- (−) CI time grows with the suite — budgeted (PR < 10 min) and optimized as a first-class concern.

## Risks
Gate friction causing workarounds (skipped tests with fake issues) — mitigated by review culture + skipped-test audit in the monthly quality review.

## Migration trigger
Gate evolution (e.g., mutation testing, Pact) at Level 3 or on measured defect escape patterns.

## Review date
At each milestone (M1–M4): does the gate predict quality? Adjust with evidence.
