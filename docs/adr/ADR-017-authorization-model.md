# ADR-017: Authorization Model — Role Middleware + Object Checks, No RLS at MVP

**Status**: Accepted (Phase 1)
**Date**: 2026-07-22

## Context
AUTH-06 defines two roles; the authorization matrix defines per-action rules. Phase 1 must choose enforcement architecture: route-level roles only, layered checks, or database RLS now.

## Options considered
1. Route-level role checks only — insufficient where object ownership matters (keys, profile); explicitly discouraged by the prompt.
2. **Three layers: middleware (auth/role/scope) → use-case object checks → repository scoping. No RLS at MVP (single workspace); workspace joins present for additive Level 3 RLS.**
3. Full RLS now — one workspace makes it untestable value; operational cost (per-connection GUC management) for zero MVP benefit.
4. Policy engine (OPA/Casbin) — dependency for a 2-role model; rejected.

## Decision
Option 2, per `docs/api/07-authentication-and-authorization.md` and `docs/security/01-ui-authorization-matrix.md`.

## Rationale
- Middleware handles the uniform part (role gates); use cases handle the ownership part (key owner = principal; self-lockout guards) where business context exists.
- Role read from DB per request (not JWT) makes admin disable immediately effective — a JWT-claim role would lag until token expiry.
- ADR-009's workspace joins keep RLS a future additive step, not a retrofit.

## Consequences
- (+) Every protected action has exactly one named enforcement point (testable per matrix row).
- (+) No framework dependency; plain Go middleware.
- (−) Authorization logic lives in code — mitigated by the matrix-as-tests discipline (every row an integration test).

## Risks
Developer adds an endpoint without wiring middleware — mitigated by default-deny router pattern (routes declare auth explicitly; review checklist; matrix test coverage).

## Migration trigger
Level 3 multi-workspace: RLS policies keyed on `current_setting('app.workspace_id')` (ADR-009 path) + workspace-membership module.

## Review date
At Level 3 gate.
