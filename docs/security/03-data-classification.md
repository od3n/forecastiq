# ForecastIQ — Data Classification (Phase 1)

**Version**: 1.0
**Status**: Approved — Phase 1 Architecture
**Authority**: NFR-D07/D08; threat model; BR-08

---

## 1. Classification Tiers

| Tier | Definition | Handling |
|------|-----------|----------|
| **Public** | Intentionally published; no harm from disclosure | Attribution required where provider-derived (BR-ATTR-01) |
| **Internal** | Operational data; disclosure harms operations but not individuals | Access: authenticated+; no logging of full content |
| **Confidential** | Credentials/secrets; disclosure enables direct harm | Never logged, never in API responses, env-only storage |
| **Restricted** | Personal data (GDPR); disclosure violates privacy | Minimize; export/delete rights; retention limits |

## 2. Data Inventory

| Data | Tier | Storage | Exposure | Retention | Notes |
|------|------|---------|----------|-----------|-------|
| Rankings, metrics, methodology | Public | DB (derived tables) | API public + UI | Indefinite | ForecastIQ's own computations (BR-LIC-01) |
| Provider attribution | Public | providers table | API + UI footer | Forever | ToS obligation |
| Snapshots (normalized) | Public (via user+ endpoints) / Internal (bulk) | forecast_snapshots | user+ raw endpoints; public only via bounded /forecast-comparison | 2 y | Derived from lawfully stored inputs; ToS-gated publication (D-05) |
| Observations | Public (bounded) / Internal (bulk) | observations | same pattern | 5 y | Provenance always labeled |
| Raw provider payloads | **Internal** | Volume (gzip) | Never served; admin sees key+checksum prefix | 90 d | Licensing-sensitive; no redistribution path exists |
| Collection metadata | Internal | forecast_collections | admin endpoints | Indefinite | Error codes, latencies — operational |
| Health/operational data | Internal | Derived + admin API | admin only | Live | Never public (SEC-02 ruling) |
| Audit events | Internal | audit_events | admin only | 1 y | Contains IPs (Restricted component) |
| User email | **Restricted** | users + Supabase | Self + admin (list shows email; no auth_subject) | Until deletion | GDPR personal data |
| auth_subject | Restricted | users | Never exposed except audit context | Until deletion | Pseudonymous identifier |
| IP addresses | Restricted | audit_events | admin only | 1 y | Operational necessity documented |
| API key plaintext | Confidential | Shown once, never stored | Creator, once | — | Hash stored (argon2id) |
| API key hash | Confidential | api_keys | Never | Until deletion | |
| Provider API keys (OpenWeather) | **Confidential** | Env only | Never | Until rotation | credential_ref = name only |
| Supabase service-role key | Confidential | Env only | Never | Until rotation | Backend-only; bundle grep CI check |
| DATABASE_URL | Confidential | Env only | Never | 90 d rotation | |
| GDPR export files | Restricted | Volume | Unguessable link, 24 h | 24 h | Deleted after expiry |
| JWTs | Confidential | Browser only | Never logged | ≤ 1 h | |

## 3. GDPR Position (NFR-D08, documented)

- **Weather data is not personal data** (no natural person identified/identifiable).
- Personal data inventory: email, auth_subject, IP (audit), preferences — all in identity module.
- Rights implementation: export (AUTH-09: account JSON + created-resources list), deletion (account + keys; audit retains anonymized reference via SET NULL), rectification (profile PATCH).
- Processing basis: legitimate interest (portfolio service) + consent (registration); privacy policy at launch (NFR-CMP03).

## 4. Tier Handling Rules (binding)

| Rule | Tiers |
|------|-------|
| Logging | Public/Internal: field-allowlisted; Confidential/Restricted: never (emails → subject refs; IPs → audit table only) |
| API exposure | Confidential: structurally absent from response types; Restricted: self/admin-scoped only |
| Backups | DB dumps inherit tier (encrypted at rest + offsite); export files excluded from offsite (24 h lifecycle) |
| Cross-border | Managed vendors (EU/US with SCCs); documented in privacy policy |
| Test data | Production Confidential/Restricted data never in test environments (fixtures synthetic — `docs/testing/05-test-data-and-fixtures.md`) |

## 5. Cross-Reference

- Secrets handling detail: `docs/security/04-secrets-management.md`
- Audit: `docs/security/05-audit-requirements.md`
- Threat model: `docs/security/02-threat-model.md`
