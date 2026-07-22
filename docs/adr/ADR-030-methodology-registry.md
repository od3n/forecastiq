# ADR-030: Methodology Registry as Versioned Configuration, Not a Database Table

**Status**: Accepted (Phase 1) — ratifies reconciliation verdict (doc 04 §1)
**Date**: 2026-07-22

## Context
The prompt's table list includes `methodology_versions`. The reconciliation board evaluated a MethodologyVersion table and ruled it unnecessary for MVP. Phase 1 records the implementation decision.

## Options considered
1. `methodology_versions` table (formulas/weights in DB rows) — implies runtime-selectable methodology; MVP has exactly one active version; formulas are code, not data (a DB row cannot execute).
2. **Methodology as versioned code + configuration: registry package (`internal/analysis/methodology`) holding formula implementations, weight sets, thresholds, version constants; served by `GET /rankings/methodology`; every derived row records the version string it was computed under.**
3. Feature-flag service for weights — excluded technology (constraints §3).

## Decision
Option 2, per `docs/data/07-methodology-evolution.md` §6.

## Rationale
- Formulas must be executable and property-tested — they live in code either way; a table would only mirror metadata that the code already carries.
- Version strings on rows (`methodology_version`, `weights_version`) deliver the actual requirement: every displayed result identifies its methodology (PC-02).
- Change governance (doc 07 §7) is a code-review process — test vectors, property invariants, worked example — which a table cannot enforce.

## Consequences
- (+) Zero additional schema; `/rankings/methodology` serves the registry directly (single source of truth).
- (+) Custom weights via API are hashed (`custom:<hash>`) without any persistence machinery.
- (−) Historical methodology metadata (what 2026.1 meant) lives in docs + code history — adequate (docs are versioned in the same repo).

## Risks
None material; the risk this avoids is a table that drifts from the code it mirrors.

## Migration trigger
Per-workspace selectable methodology (Level 3) → then a table keyed by (workspace, version) referencing registry names.

## Review date
At Level 3 gate.
