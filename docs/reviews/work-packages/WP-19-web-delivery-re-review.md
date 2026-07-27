# ForecastIQ — WP-19 Web Session Auth: DRB Confirmatory Re-Review

**Review date**: 2026-07-27
**Work package**: WP-19 (web slice) — Web Session Auth (PR #35, `feature/wp19-web-session-auth`)
**Prior review**: WP-19-web-delivery-review.md — REJECTED on `0e4739b` (DRB-WP19W-001..002)
**Reviewed SHA**: `ac119c0`
**Decision**: **ACCEPTED**

---

## 1. Verification of evidence

| Check | Result |
|-------|--------|
| Commit identity: local branch == remote == CI head | ✅ all `ac119c0` |
| Seven jobs green (incl. frontend-checks) | ✅ first run |
| `tsc --noEmit`, eslint | ✅ clean |
| vitest | ✅ 50/50 (was 42; +8 auth incl. the new backslash cases) |

## 2. Finding closure

| Finding | Status | Resolution |
|---------|--------|-----------|
| 001 (M, security) open-redirect via backslash | ✅ | `safeReturnPath` now also rejects `raw.includes("\\")`; applied at the source (`signin/page.tsx:155`) so both the Supabase (`router.push`) and dev (`location.assign`) forms are covered; regression test asserts `"/\\evil.example"` and `"\\\\evil.example"` → `/` |
| 002 (L) menu a11y | ✅ | `aria-haspopup` dropped (kept `aria-expanded` — it's a disclosure, not a menu); Escape now calls `triggerRef.current?.focus()` to restore focus to the trigger |

## 3. Verified sound (carried from first review)

Dev-token-in-prod-bundle property HOLDS/strengthened (no env-injected token in
any build; token only via verified sign-in); per-request token resolution (no
stale closure); hydration-safe (all localStorage in useEffect/handlers/fetcher,
`useSession` loading placeholder); provider-neutral env-presence selection
(zero-code revert); clean merge (no `devAuthHeaders`/duplicate-import residue).

## 4. Process note

The editor failed to persist several backslash-heavy edits and, on two files
(`session.ts`, `auth.test.tsx`), a failed save duplicated the file tail — caught
by `tsc` (TS1128), fixed by trimming the stray lines; re-verified green. Fixes
were completed via `perl`/`sed`. No functional impact on the delivered code.

## 5. Decision

**ACCEPTED.** Both findings closed; seven jobs green on `ac119c0`. PR #35 ready
to merge.

**This closes the WP-22…WP-27 + WP-19-web DRB queue** — all eight open PRs
processed (reject → fix → re-review → accept → merge), except the explicitly
tracked follow-ons: **WP-26b** (performance completion) and the ADR-033 §4
security items (CF-Connecting-IP mapping + EC2 security-group restriction to
Cloudflare ranges).
