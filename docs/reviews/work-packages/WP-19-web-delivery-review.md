# ForecastIQ — WP-19 Web Session Auth: Delivery Review Board

**Review date**: 2026-07-27
**Work package**: WP-19 (web slice) — Web Session Auth (PR #35, `feature/wp19-web-session-auth`)
**Note**: distinct from the backend `WP-19-delivery-review.md` (auth/authz core, PR #17, accepted 2026-07-24). This reviews the frontend session/sign-in slice.
**Reviewed SHA**: `0e4739b` (post main merge-up)
**Decision**: **REJECTED — 1 Medium security finding (open redirect); 1 Low a11y**

---

## 1. Context

The cleanest branch in the WP-22…WP-27 queue: `tsc` clean, 8/8 new tests pass,
merge resolution verified clean (no leftover `devAuthHeaders`, no duplicate
`authHeaders` imports). The DRB-WP23L-004 security property is not just
preserved but **strengthened** (see §4). One genuine security bug blocks
acceptance; everything else is sound.

## 2. Findings

### Medium (security — blocking)

**DRB-WP19W-001 (M)** — `safeReturnPath` open-redirect bypass via backslash.
The guard rejects `//host` and non-`/` values but browsers normalize `\` to
`/`, so `?return=/\evil.example` passes (`startsWith("/")` true, `startsWith("//")`
false) and `window.location.assign("/\\evil.example")` navigates to
`https://evil.example` — a phishing vector on the sign-in flow (victim signs in
on the real site, lands on the attacker's). Verified: `session.ts:84-87`;
consumed at `signin/page.tsx:110` (dev form `location.assign`) and `:39`
(Supabase `router.push`). The new test suite tests this guard but misses the
bypass. Fix: also reject `raw.includes("\\")` (or parse via `new URL(raw,
origin)` and require same-origin) and add the backslash case to the rejection
test.

### Low

**DRB-WP19W-002 (L)** — Account menu a11y: `aria-haspopup="true"` promises
`menu` semantics (arrow-key navigation) that the plain link/button `div`
doesn't implement; and Escape closes the popup without returning focus to the
trigger (focus falls to `<body>`). Verified: `AppHeader.tsx:44,54`. Fix: drop
`aria-haspopup` (keep `aria-expanded` — this is a disclosure, not a menu), and
`triggerRef.current?.focus()` after `setOpen(false)` on Escape.

### Info (no action)

**DRB-WP19W-003 (Info)** — With Supabase env unset, a production build shows the
dev-token sign-in form. This **fails safe**: the token is verified against
`GET /me` before storage, and release builds only wire the JWKS verifier
(`devauth` is `//go:build !release`; composition root selects it only under
`cfg.AuthDevMode`), so a dev token grants nothing in prod. Misconfiguration
visibility only; the migration plan's `/auth/config` endpoint removes it later.

## 3. Scope coverage

| Item | Status |
|------|--------|
| Sign-in required | ✅ route-level gating (admin layout + settings → 401 redirect with `?return=`); public reads intentionally remain public per the in-diff UI spec §3.1 |
| Account dropdown (email → Settings/Admin/Sign out) | ✅ role-gated Admin via `/me`; minor a11y gaps (002) |
| Session management | ✅ per-request token resolution (no stale closure); signed-out goes bare; Supabase refresh; `onAuthStateChange` unsubscribed |
| Hydration safety | ✅ all localStorage in useEffect/handlers/fetcher; `useSession` starts "loading" with placeholder — no signed-out flash |
| Provider-neutral / zero-code revert | ✅ env-presence selection (Supabase vs dev); removing env reverts with no code change |
| Tests | ✅ real, 8/8 (redirect guard, verify-before-store, bearer attach, axe); gaps: no useSession/dropdown coverage, misses the 001 bypass |

## 4. Dev-token-in-prod-bundle property — HOLDS (strengthened)

`NEXT_PUBLIC_DEV_TOKEN` fully removed (docker-compose, web env docs, hooks.ts);
`session.ts` reads no `process.env` token at all (only browser-safe Supabase
public config). No build-time token source exists, so nothing token-like can be
inlined into a production bundle. The dev token now enters only via user input
on `/auth/signin`, verified against `GET /me` before persistence. No token
logged. This supersedes DRB-WP23L-004 with a stronger invariant: no
environment-injected session in any build.

## 5. Decision

**REJECTED.** DRB-WP19W-001 (open redirect) is a real security finding on the
auth flow and must be fixed with a regression test; 002 (a11y) fixed with it.
Re-review requires green CI (incl. frontend-checks) on the new SHA. Everything
else — session core, gating, hydration safety, zero-code-revert, the
prod-bundle token property — is verified sound.
