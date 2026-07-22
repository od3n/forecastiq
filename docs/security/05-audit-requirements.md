# ForecastIQ — Audit Requirements (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: NFR-SEC11; AUTH-07; `docs/security/01-ui-authorization-matrix.md` §7; domain model (AUDIT_EVENT)

---

## 1. Principles

1. **Every administrative mutation and every auth event is audited** — no exceptions, no best-effort (audit write failure fails the command's transaction).
2. Audit rows are **immutable** (trigger-enforced; no UPDATE/DELETE path; retention purge is the only removal, after 1 y, via maintenance exemption).
3. Audit captures **who, what, when, from where** — actor (user_id), action, resource (type + id), details (JSONB, sanitized), ip_address.
4. Audit is **same-transaction** (no outbox — single process, single DB; participation in the command tx is trivially consistent).
5. Read surface: admin-only endpoint (`GET /admin/audit-events`, cursor pagination); no user self-audit in MVP (GDPR export includes own events).

## 2. Action Registry (binding, stable names)

| Category | Actions | Details captured |
|----------|---------|------------------|
| auth | `auth.login`, `auth.login_failed`, `auth.registered`, `auth.email_verified`, `auth.password_reset_requested`, `auth.password_reset_completed`, `auth.logout` | subject ref, IP (login events via webhook from Supabase where available; app-side for API-key auth) |
| api_key | `api_key.created`, `api_key.revoked` | key_prefix, scopes, rate_limit (never the key or hash) |
| user | `user.preferences_updated`, `user.disabled`, `user.enabled`, `user.deleted`, `user.export_requested`, `user.export_completed` | changed field names (not sensitive values), actor vs target |
| admin_user | `admin.user_disabled`, `admin.user_enabled`, `admin.user_deleted`, `admin.export_requested` | actor, target, scope summary |
| location | `location.created`, `location.updated`, `location.status_changed` | field diffs (name/timezone/status), dedup override flag |
| provider | `provider.status_changed`, `provider.config_updated` | old/new status; schedule diff; credential change as "credential_ref rotated" (never the value) |
| collection | `collection.triggered`, `collection.replayed` | provider, location, actor; replay: source → new collection ids |
| analysis | `rankings.recompute_triggered` | scope filters, methodology_version |
| system | `system.migration_applied` (deploy pipeline writes) | migration ids, version |

New actions require: registry entry in this doc + code constant + test asserting emission.

## 3. Sanitization Rules (what NEVER goes in details)

- Credential values, key plaintext/hashes, credential_ref values
- Tokens of any kind
- Full request/response bodies (field names and non-sensitive values only)
- Email addresses (use auth_subject reference; admin audit UI shows email from join for readability — the audit row itself stores user_id only)
- Raw provider payloads

## 4. Storage and Lifecycle

| Attribute | Specification |
|-----------|---------------|
| Table | `audit_events` (domain model) |
| Retention | 1 year (NFR-D04); monthly bounded purge (maintenance exemption, logged) |
| Immutability | BEFORE UPDATE OR DELETE trigger (no exemption except retention purge GUC) |
| Indexes | `(created_at DESC)`; `(resource_type, resource_id, created_at DESC)` |
| Volume | ~100 rows/day → ~37K/year (trivial) |
| GDPR | user_id ON DELETE SET NULL (event preserved, actor anonymized); own events included in AUTH-09 export |

## 5. Auth Event Sourcing

Supabase performs login/registration/reset; ForecastIQ captures:
- **App-observable events**: first-seen provisioning (`auth.registered` local equivalent), API-key auth usage (throttled: audit once per key per day for routine success; failures always), admin disable/delete actions.
- **Webhook ingestion (if configured)**: Supabase auth webhooks → signed endpoint → audit rows for login/failed-login/reset events. MVP: webhook receiver implemented (small, high value for security visibility); signature verification mandatory (HMAC, timing-safe); failures logged but non-blocking (auth flow must not depend on our audit availability).

## 6. Access and Review

| Activity | Frequency |
|----------|-----------|
| Admin UI review (S-14) | On demand |
| Security review: failed-login patterns, admin action spot-check | Monthly |
| Anomaly alert: admin actions outside business hours (configurable) | Continuous (log-based alert) |
| Incident investigation queries | As needed (resource + time filters) |

## 7. Test Requirements

- Every action in the registry has an integration test asserting emission with correct fields (authorization matrix actions × audit assertion).
- Immutability: test that UPDATE/DELETE on audit_events raises.
- Sanitization: test that credential-touching operations produce sanitized details (grep details for known secret fixtures).

## 8. Cross-Reference

- Action matrix source: `docs/security/01-ui-authorization-matrix.md` §7
- Table DDL: `docs/data/03-table-design.md` §6
- Testing: `docs/testing/02-testing-strategy.md`
