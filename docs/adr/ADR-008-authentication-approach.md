# ADR-008: Managed Authentication via Supabase Auth

**Status**: Accepted (Phase 0 Amendment)
**Date**: 2026-07-22

## Context
Blocker 6: Phase 0 defined login but no registration, password policy, reset flow, or
session lifecycle, and implied app-managed password storage. The amendment required
choosing the simplest secure approach for a public portfolio MVP and comparing
application-managed auth against managed providers.

## Options considered

| Option | Effort (MVP) | What we own | Fit |
|--------|--------------|-------------|-----|
| App-managed (bcrypt, own reset/verification email, session table) | 8–12 d + ongoing security surface | Everything incl. password hashes, email delivery, brute-force defense | Maximum control; maximum liability for a 1–2 person team |
| **Supabase Auth** | 3–4 d | JWT verification (JWKS) + user mapping table only | Free tier aligns with managed-Postgres choice; email flows included |
| Auth0 | 4–6 d | Token verification + rules config | Mature; free tier limits (7.5K MAU) fine, but pricing cliff risk and heavier config |
| Clerk | 4–6 d | Token verification | Excellent DX; component library React-centric; pricing cliff risk |

## Decision
Supabase Auth. The ForecastIQ `users` table stores `auth_subject` (Supabase user id),
email, role, status — **no password hashes**. The Go backend verifies JWTs via JWKS.
Registration: self-service with mandatory email verification. Password policy:
minimum 12 characters (NFR-SEC15). Refresh-token rotation with theft detection;
concurrent sessions allowed (NFR-SEC16). Brute-force/rate limiting on auth flows
provided by the managed service plus app-level limiting on auth-adjacent endpoints.

## Rationale
- The team should spend its security budget on the novel parts (API keys, scopes,
  audit), not re-implementing password reset correctly.
- Supabase consolidates auth + (optionally) managed Postgres in one vendor relationship
  at $0 for MVP scale.
- Standard JWT/JWKS verification keeps the backend portable: switching providers later
  changes the JWKS source and subject mapping, not the authorization model.

## Consequences
- (+) Registration, verification, reset, session rotation, brute-force defense: done.
- (+) No credential storage in our DB → smaller breach blast radius, simpler GDPR.
- (−) Vendor dependency (R-14): mitigated by standards-based verification and the
  subject-mapping design; outage degrades to public-data-only mode.
- (−) Account disable/delete requires admin calls to the vendor API (automated in
  ADMIN-05).

## Migration trigger
Revisit when: MAU approaches free-tier limits, OR pricing changes materially, OR
enterprise SSO (SAML/OIDC federation) is required at Level 3 (then evaluate Auth0/
Clerk/WorkOS). Migration path: new provider's subjects added as a second column,
dual-verification window, then cutover.

## Review date
At Level 3 gate and on any vendor pricing change.
