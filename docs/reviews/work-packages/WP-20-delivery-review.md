# ForecastIQ — WP-20 Frontend Foundation: Delivery Review Board

**Review date**: 2026-07-24
**Work package**: WP-20 — Frontend Foundation
**Reviewed SHA**: `0837383be8fd8a82fb77ffb4b8ef5e3da919c740` (`0837383`)
**Decision**: **ACCEPTED**

---

## 1. Verification of evidence

| Check | Result |
|-------|--------|
| Commit identity: local HEAD == remote == CI head | ✅ all `0837383` |
| CI run **30116246034** (CI, six backend jobs) | ✅ success (first run) |
| CI run **30116246139** (Frontend, `frontend-checks`) | ✅ success (first run) |
| Seven jobs green, none skipped/cancelled | ✅ backend-checks, backend-integration, migrations, api-contract, security, image, frontend-checks |
| Dependency gate: WP-15 Accepted | ✅ (registry line 15) |

## 2. Scope review (all 8 items verified)

- **Next.js static export** (`output: 'export'`; React 19; 103 kB shared < 200 kB budget).
- **Design tokens**: exact `docs/ui/02` §1 values as CSS custom properties (colors, typography, spacing, radius, elevation, motion); Inter + JetBrains Mono via `next/font`.
- **Generated API client**: `openapi-typescript` → `generated.ts` (drift-gated in CI; regeneration produces no diff).
- **Envelope rendering primitives**: FreshnessBadge (all four states, non-color text), StaleBanner (`role="status"`), PartialWarnings (`role="note"`), EmptyState, ErrorPanel (`role="alert"`, focus-to-retry, request_id).
- **S-15 error boundary**: `error.tsx` with request_id + Retry (uses ErrorPanel); `not-found.tsx` (404); `global-error.tsx` (fallback).
- **Supabase auth (S-08)**: signin/signup/reset/verify; SDK-backed forms; no-enumeration reset; `null + notice` when unconfigured (dev without project); labelled inputs; 44px targets.
- **CSV export utility**: metadata header block (§5 binding format), formula-injection guard (= + - @ TAB CR neutralized), null → empty cells, attribution footer.
- **axe-core CI**: vitest + jest-axe on every primitive (9/9); `frontend-checks` job runs tests + axe.

## 3. Architecture + quality assessment

- **Separation**: `web/` is a self-contained Next package (own `package.json`, lockfile, `.gitignore`); path-aware CI only triggers on `web/**`. The six backend jobs are unchanged and unaffected. No cross-contamination.
- **Security**: npm audit 0 vulnerabilities (Next 15.5.21 + postcss/sharp overrides); service-role key grep on the static bundle (empty = no leak); Supabase anon key only (browser-safe/RLS-scoped); formula-injection neutralization tested.
- **a11y (WCAG 2.1 AA foundations)**: skip-link, visible focus, `prefers-reduced-motion`, non-color status channels (text labels always present alongside color dots/pills), axe CI from the first component.
- **Bundle**: shared First-Load JS 103 kB (well within the 200 KB budget); auth screens' Supabase SDK is code-split (169 kB per-route, not shared).
- **Design tokens faithful**: every hex, px, and font name matches doc 02 §1 exactly (verified by reading the committed globals.css).

## 4. Findings

No Critical/High/Medium finding. One informational note:

**DRB-WP20-001 (Low, informational, non-blocking)**: Next 15 / React 19 was a dependency-driven decision (Next 14.x carries residual high advisories only patched in 15.5.x). React 19 is stable but newer than the original "React 18" assumption implicit in WP-20's original planning. No functional impact; testing-library and axe-core work correctly. Documented in the implementation report.

## 5. Decision

**ACCEPTED.** WP-20 delivers the complete frontend foundation — static-export app shell, design tokens, generated API client, Supabase auth screens, envelope/freshness/warning/error rendering primitives, CSV export utility, and axe-core a11y CI — all CI-verified on `0837383` (seven jobs green including the new path-aware `frontend-checks`).

**Accepted Implementation SHA `0837383`.** PR #20 ready to merge to `main`. **WP-21 (Core MVP Screens)** becomes eligible.
