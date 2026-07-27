# ForecastIQ — Auth Provider Migration Plan (Supabase → Self-Hosted)

**Version**: 1.0
**Status**: Proposed — future plan (not scheduled; no GO issued)
**Authority**: ADR-008 (authentication approach); `docs/planning/05-implementation-work-packages.md` (WP-19/WP-19b delivered scope)

## Summary

ForecastIQ uses Supabase **only for auth** (GoTrue). Postgres/TimescaleDB, storage, and all user records are already self-hosted; `users.auth_subject` merely stores the provider's JWT `sub`. A migration therefore touches 4 backend surfaces (JWKS verifier config, admin-propagation adapter, webhook receiver, env config) and 4 frontend files, plus a scripted user-data sync and SMTP setup.

The plan is built around **operator-selectable providers and mechanical revert**: a Phase 0 provider-switch abstraction (`FIQ_AUTH_PROVIDER` profiles, a dual-accept multi-issuer verifier, a runtime auth-config endpoint) plus an automated one-directional sync/verify/freeze script. One provider is authoritative at a time; switching (either direction) is `freeze → sync → verify → flip env → unfreeze`, ~10 minutes, without signing users out.

**Recommended target: self-hosted GoTrue** (Supabase's own auth server, Docker image `supabase/gotrue`). Rationale:

- Identical Admin API (`/admin/users`) → the existing `supabaseadmin` adapter works as-is with a base-URL change
- Identical JWT claims and JWKS behavior → verifier unchanged
- Identical webhook event types → `webhookActions` map unchanged
- Native bcrypt password import → **no forced password resets**
- `supabase-js` (`auth-js`) works pointed at a bare GoTrue URL → minimal frontend changes

Keycloak is documented as the alternative in the final section (choose it only if SSO/SAML/enterprise IdP federation is a near-term requirement; it costs a new admin adapter, webhook rework, frontend OIDC rewrite, and forced password resets unless a custom hash provider is written).

## Phase 0 — Provider-switch abstraction (enables cutover AND revert)

Makes the auth provider an operator-selectable deployment choice with a rule: exactly one provider is authoritative (accepts signups/password changes) at a time.

- **Provider profiles**: `FIQ_AUTH_PROVIDER=supabase|gotrue` selects a bundled profile (JWKS URL, issuer, audience, admin API base + key, webhook secret) in `internal/platform/config/config.go`, following the existing dev/prod verifier-selection pattern in `cmd/forecastiq/app.go`. Individual `FIQ_AUTH_*` vars remain as overrides.
- **Multi-issuer verifier (dual-accept)**: a small `multiverifier` composite implementing `ports.TokenVerifier` that tries each configured verifier in order (authoritative first). Configured via an optional secondary JWKS URL/issuer (`FIQ_AUTH_SECONDARY_JWKS_URL`/`FIQ_AUTH_SECONDARY_ISSUER`); unset ⇒ single-verifier behavior, zero overhead. Transition-only by convention: enabled for the cutover window and removed after.
- **Runtime auth-config endpoint**: public `GET /v1/auth/config` returning `{ provider, auth_url, anon_key }`. The static-export frontend fetches this at load instead of baking `NEXT_PUBLIC_*` at build time — so switching providers no longer forces a frontend rebuild. Falls back to `NEXT_PUBLIC_*` env when the endpoint is unreachable (local dev).
- **Admin visibility (read-only)**: the admin dashboard health section shows the active provider, JWKS reachability, and whether dual-accept is on. No runtime toggle — the switch is a deliberate env change (a live toggle would make both providers writable at once, splitting the user base across providers with bidirectional, unbounded drift).

## Phase 1 — Stand up self-hosted GoTrue

- Add a `gotrue` service to `docker-compose.yml` (image `supabase/gotrue:v2.x`), with its own dedicated Postgres database/schema (`auth`) — either a new DB in the existing Postgres container or a separate instance. Do NOT share the app schema.
- Key GoTrue env: `GOTRUE_JWT_ALG=RS256` (asymmetric keys so JWKS keeps working; Supabase-hosted projects use the same), `GOTRUE_SITE_URL=<web origin>`, `GOTRUE_MAILER_AUTOCONFIRM=false`, `GOTRUE_DISABLE_SIGNUP=false`, external email confirm/recovery URL paths matching current web routes (`/auth/*`).
- Production deploy: new systemd unit + Caddy reverse-proxy route (e.g. `auth.<domain>`), mirroring the existing infra pattern.
- Verify `https://auth.<domain>/.well-known/jwks.json` serves the signing key.

## Phase 2 — Email (SMTP) configuration

Supabase currently sends verification + password-reset emails; self-hosted GoTrue needs SMTP. The app itself sends no email — nothing in Go/Next.js changes.

- Configure GoTrue: `GOTRUE_SMTP_HOST/PORT/USER/PASS`, `GOTRUE_SMTP_ADMIN_EMAIL`, `GOTRUE_SMTP_SENDER_NAME`, and the four mail templates (confirmation, recovery, magic link, email change) — port the current Supabase template content.
- Provider options (pick one):
  1. **Resend** (recommended: simple SMTP relay, generous free tier, good deliverability)
  2. **AWS SES** (cheapest at volume; requires domain verification + production-access request)
  3. **Postmark** (best transactional deliverability; paid)
  4. Self-hosted SMTP (not recommended: deliverability burden)
- DNS: add SPF, DKIM, DMARC records for the sending domain via the existing Terraform Cloudflare config.
- Smoke-test signup-confirmation and password-recovery emails against a staging GoTrue before any cutover.

## Phase 3 — User data migration (automated sync script)

Users already live in the local `users` table keyed by `auth_subject` (Supabase user id = JWT `sub`). User `id`s are preserved verbatim on import, so **`auth_subject` values never change** — no app-DB rewrite needed. Both providers speak the same `auth` schema (GoTrue), so the sync is a dumb one-directional copy, not a merge.

Deliverable: `deploy/scripts/auth-sync.sh` (pattern of the existing `deploy/scripts/`), driven by two connection strings (`AUTH_SYNC_SOURCE_DSN`, `AUTH_SYNC_TARGET_DSN`; Supabase side uses the project's direct Postgres connection string). Three subcommands:

- **`sync --from <src> --to <dst>`** — idempotent, one-directional copy:
  - Exports `auth.users` (`id`, `email`, `encrypted_password` bcrypt, `email_confirmed_at`, `banned_until`, `created_at`, `raw_user_meta_data`) + `auth.identities` via `pg_dump`/`COPY`.
  - Imports as **upsert `ON CONFLICT (id)`** — unchanged rows are no-ops; safe to re-run anytime. Sessions/refresh tokens are never copied (users re-login once at cutover).
  - Direction is a flag: cutover is `--from supabase --to gotrue`; the pre-revert catch-up is the same command reversed.
  - May be cron'd (old → new) in the days before cutover so the freeze-window delta sync takes seconds. Never run bidirectionally or post-cutover toward the retired provider except as the explicit pre-revert catch-up.
- **`verify`** — the cutover gate. Compares three sets: source provider ↔ target provider ↔ local `users.auth_subject` (excluding `dev|*`). Reports missing-on-target, email mismatches, password-hash drift, and ban-status drift (cross-check local `users.status = 'inactive'` against `banned_until`). Exits non-zero on any gap; cutover and revert are both gated on a clean `verify`.
- **`freeze` / `unfreeze`** — toggles signups off/on on the authoritative side (Supabase Admin API / `GOTRUE_DISABLE_SIGNUP`), bounding the sync window so the copy is provably complete.

## Phase 4 — Backend changes (Go)

All auth flows go through ports; changes are config + naming only.

- **JWKS verifier** (`adapters/auth/jwks`): no code change. Env-only: `FIQ_AUTH_JWKS_URL=https://auth.<domain>/.well-known/jwks.json`, `FIQ_AUTH_ISSUER=https://auth.<domain>`, audience per GoTrue config (`authenticated`).
- **Admin propagation** (`adapters/auth/supabaseadmin`): GoTrue serves the same `/admin/users/{id}` endpoints. Change: base path handling (self-hosted GoTrue has no `/auth/v1` prefix — make the path prefix configurable or strip it), and auth header uses a GoTrue admin JWT (`GOTRUE_JWT` service key) instead of the Supabase service-role key. Rename package/port to provider-neutral `authadmin` / `ports.AuthAdmin` (mechanical rename; interface unchanged).
- **Webhook receiver**: GoTrue emits the same event types already mapped in `internal/identity/webhook_service.go`; configure `GOTRUE_WEBHOOK_URL` + `GOTRUE_WEBHOOK_SECRET` to point at the existing receiver with `FIQ_AUTH_WEBHOOK_SECRET`. Verify GoTrue's signature scheme version against the receiver's HMAC check in the integration test rig (`test/integration/user_lifecycle_test.go` pattern) and adjust the edge verification if the header format differs.
- **Config** (`internal/platform/config/config.go`): rename `FIQ_SUPABASE_URL`/`FIQ_SUPABASE_SERVICE_ROLE_KEY` → `FIQ_AUTH_ADMIN_URL`/`FIQ_AUTH_ADMIN_KEY` (keep old names as deprecated aliases for one release); update `.env.example`.
- Update comments/docs referencing "Supabase" in identity package docs, ADR-008 (append a superseding ADR rather than rewriting), and `seed.go` bootstrap-admin comment.

## Phase 5 — Frontend changes (Next.js)

`supabase-js` works against bare GoTrue, so changes are minimal:

- `web/lib/auth/supabase.ts`: keep `createClient` but point `NEXT_PUBLIC_SUPABASE_URL` at the self-hosted GoTrue origin; the anon key becomes a GoTrue-issued anon JWT signed with the same key. Rename file/env to provider-neutral (`lib/auth/client.ts`, `NEXT_PUBLIC_AUTH_URL`/`NEXT_PUBLIC_AUTH_ANON_KEY`) in the same change.
  - Alternative (cleaner long-term): replace `supabase-js` with direct GoTrue REST calls (`/token?grant_type=password`, `/signup`, `/recover`) + a small session store — only if dropping the dependency is desired; not required for cutover.
- Signin/signup/reset pages: unchanged API surface (`signInWithPassword`, `signUp`, `resetPasswordForEmail`).
- `web/lib/auth/session.ts`: unchanged (reads SDK session).
- Update `web/.env.example` and CI env wiring.

## Phase 6 — Cutover & rollback

Cutover runbook (~15 min, mostly watching script output):

1. Days before: deploy GoTrue + SMTP alongside Supabase; optionally cron `auth-sync.sh sync --from supabase --to gotrue` so the final delta is tiny. Enable dual-accept (secondary JWKS = GoTrue) via Phase 0 config.
2. `auth-sync.sh freeze` → `sync` → `verify` (gate: must exit clean).
3. Flip `FIQ_AUTH_PROVIDER=gotrue` (JWKS/admin/webhook profile) and restart the backend; the frontend picks up the new provider from `/v1/auth/config` — no rebuild required.
4. `auth-sync.sh unfreeze` (now against GoTrue as the authoritative side).
5. Existing sessions keep working through dual-accept (Supabase-issued JWTs still verify); users transition to GoTrue-issued tokens as sessions naturally refresh/re-login. Passwords unchanged.
6. After a 2-week window with Supabase kept alive but frozen: disable dual-accept, delete the Supabase project, remove deprecated env aliases.

Revert (mechanical, same tooling, any time within the window):

1. `auth-sync.sh sync --from gotrue --to supabase` — catch-up copy of post-cutover signups/password changes (drift is one-directional and bounded because Supabase stayed frozen).
2. `auth-sync.sh verify` (gate) → flip `FIQ_AUTH_PROVIDER=supabase` back → unfreeze Supabase signups.
3. Dual-accept keeps GoTrue-issued sessions valid through the flip. Total: two commands + one env change, ~10 minutes.

## Test Plan

- Unit: `jwks` tests already cover generic JWKS; add `multiverifier` tests (authoritative-first ordering, secondary fallback, both-fail ⇒ `ErrInvalidToken`, expired precedence); a GoTrue-shaped webhook signature test; adapt `supabaseadmin` tests to the configurable path prefix; provider-profile resolution tests in `config`.
- Integration (`test/integration`): existing rig uses `devauth` + noop admin — unchanged; add one webhook-format test for the GoTrue signature header and a `/v1/auth/config` contract test.
- Script: `auth-sync.sh` rehearsed against two local Postgres instances in CI or a staging pair — `sync` idempotency (second run = zero changes), `verify` failure on a seeded gap, freeze/unfreeze round-trip.
- Staging E2E before cutover: signup → confirmation email → signin → API call with bearer → admin ban (propagation) → password reset email → reset flow.
- Post-cutover checks: JWKS fetch metrics, login success rate, webhook audit rows appearing, admin ban/delete round-trip on a test account.

## Alternative: Keycloak (only if SSO/enterprise federation needed)

Extra cost over GoTrue: new `authadmin` adapter against the Keycloak Admin REST API (`PUT /admin/realms/{realm}/users/{id}` with `enabled:false` for ban, `DELETE` for delete); webhook replaced by a Keycloak event-listener SPI or polling the events API (the HMAC receiver format doesn't match); frontend rewritten to OIDC Authorization Code + PKCE (drop `supabase-js`, use `oidc-client-ts` or keycloak-js) — all three auth pages redirect to Keycloak-hosted screens; user import via Keycloak's partial-import JSON, but bcrypt hashes require mapping to Keycloak's credential format (supported: bcrypt is accepted in credential import) — verify on a sample before committing, otherwise forced resets. `auth_subject` values change to Keycloak user ids unless ids are preserved via import (Keycloak allows explicit `id` in import JSON — preserve them). Email: same SMTP options, configured per-realm.

## Assumptions

- Supabase-hosted project uses RS256/ES256 asymmetric signing (required by the current JWKS verifier — verify in project settings; if legacy HS256, rotate to asymmetric on Supabase first, before migration).
- Supabase allows `auth` schema export via `pg_dump` on the current plan.
- A ~15-minute signup freeze window is acceptable for cutover (and for a revert, if exercised).
- Frontend can tolerate one extra startup fetch (`/v1/auth/config`) before initializing the auth client; env fallback covers local dev.
- No magic-link or OAuth social logins are currently enabled (only email+password flows exist in the UI).
