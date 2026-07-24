# ForecastIQ — WP-20 Frontend Foundation: Implementation Report

**Version**: 1.0
**Implementation date**: 2026-07-24
**Work package**: WP-20 — Frontend Foundation
**Authority**: `docs/planning/05-implementation-work-packages.md` §WP-20; `docs/ui/02` (design system) / `06` (state contracts) / `07` (accessibility) / `05` (S-08/S-15); `docs/api/02` (envelope §1-5); `docs/delivery/01-02-03`; ADR-013 (static export), ADR-023 (`web/` monorepo)
**Branch**: `feature/wp20-frontend-foundation` (base: `main` `3ccc8de`)
**Status**: **Implementation Complete — Not Accepted** (Delivery Review Board transition only)

> The dashboard **foundation** — app shell, tokens, API client, auth, rendering primitives, CSV, a11y CI. The 15 screens + states land in WP-21.

---

## 1. Executive summary

The `web/` Next.js dashboard is scaffolded across four green slices:

- **Slice 1 (`e9a4678`)** — Next.js 15 **static export** (`output: 'export'`; React 19; TS strict); design tokens as CSS custom properties (exact `docs/ui/02` §1 values; Inter + JetBrains Mono via `next/font`); app shell (root layout, global header, attribution-footer primitive, Overview placeholder); WCAG foundations (skip-link, visible focus, `prefers-reduced-motion`); **path-aware frontend CI** (`.github/workflows/frontend.yml`, `web/**`).
- **Slice 2 (`8c77a60`)** — **generated API client** (`openapi-typescript` → `web/lib/api/generated.ts`, CI drift-gated) + hand-written envelope types + `apiGet` (X-Request-Id capture; `ApiError` carries the RFC 7807 problem + request_id); **envelope rendering primitives** (FreshnessBadge, StaleBanner, PartialWarnings, EmptyState, ErrorPanel) with non-color channels; **S-15 error boundary** (`error.tsx` with request_id + retry, `not-found.tsx`, `global-error.tsx`).
- **Slice 3 (`7407627`)** — **Supabase SDK auth (S-08)**: `/auth/signin|signup|reset|verify` custom forms calling the SDK (C-17; no app-managed passwords); no-enumeration reset; browser-safe `NEXT_PUBLIC_SUPABASE_*` config (null + notice when unconfigured).
- **Slice 4 (`bfec119`)** — **CSV export utility** (conventions §5 metadata block + formula-injection guard + null→empty) + **axe-core a11y CI** (vitest + Testing Library + jest-axe on every primitive; CSV unit tests).

## 2. Scope reconstruction (§WP-20)

| # | Scope item | This package | Result |
|---|-----------|--------------|--------|
| 1 | Next.js init (static export) | `output: 'export'`, CDN-ready `out/` | ✅ |
| 2 | Generated API client (OpenAPI) | `openapi-typescript` → `generated.ts` (drift-gated) + typed `apiGet` | ✅ |
| 3 | Supabase SDK auth flow (S-08) | signin/signup/reset/verify | ✅ |
| 4 | Design tokens per design system | CSS custom properties, exact §1 values | ✅ |
| 5 | Envelope/warnings/freshness primitives | FreshnessBadge/StaleBanner/PartialWarnings/EmptyState + AttributionFooter | ✅ |
| 6 | Error-boundary + request_id (S-15) | `error.tsx`/`not-found.tsx`/`global-error.tsx` + ErrorPanel | ✅ |
| 7 | CSV export utility (conventions §5) | `lib/csv/export.ts` (formula-safe) | ✅ |
| 8 | axe-core CI from first screen | vitest + jest-axe on all primitives | ✅ |

## 3. Design notes + deviations

- **Styling**: CSS custom properties + CSS Modules (doc 02 §14.7 allows Tailwind *or* CSS Modules) — no Tailwind dependency, tokens explicit, lean bundle.
- **Next 15 / React 19 (dependency-driven)**: the audit gate (`npm audit --omit=dev --audit-level=high`) required moving off Next 14.x (residual highs only patched in 15.5.21); `postcss`/`sharp` overrides clear the remaining transitive highs (both unused at runtime in a static export). `supabase-js` pinned 2.110.8. Result: **0 vulnerabilities**.
- **a11y scope**: axe runs on the **primitives** (foundation); document-scoped rules (landmark/region/single-H1) and color-contrast (needs layout, not evaluated in jsdom) are deferred to the full-page screen checks in WP-21. Non-color channels (text labels beside every dot/badge) are enforced now (A-02/§3).
- **Auth completeness**: SDK-backed sign-in/up/reset/verify; the password-recovery-token landing + session-aware header (avatar/role-gated nav) arrive with the screens (WP-21). No real Supabase project in dev/CI, so forms degrade to a "not configured" notice and build/tests run without it.
- **Path-aware CI**: a separate `Frontend` workflow triggers only on `web/**`; the six backend jobs (ci.yml) are untouched and still run on every PR.

## 4. Files (web/)

- **Config**: `package.json` (+lockfile), `tsconfig.json`, `next.config.mjs`, `.eslintrc.json`, `.gitignore`, `.env.example`, `vitest.config.ts`, `next-env.d.ts`.
- **App**: `app/layout.tsx`, `app/page.tsx`, `app/globals.css` (tokens), `app/error.tsx`, `app/not-found.tsx`, `app/global-error.tsx`, `app/auth/{layout,signin,signup,reset,verify}` + `auth.module.css`.
- **Components**: `AppHeader`, `AttributionFooter`, `FreshnessBadge`, `StaleBanner`, `PartialWarnings`, `EmptyState`, `ErrorPanel` (+ CSS modules).
- **Lib**: `lib/api/{generated.ts,types.ts,client.ts}`, `lib/format.ts`, `lib/auth/{supabase.ts,messages.ts}`, `lib/csv/export.ts`.
- **Tests**: `test/{setup.ts,vitest.d.ts,a11y.test.tsx,csv.test.ts}`.
- **CI**: `.github/workflows/frontend.yml`.

## 5. Local gate

- `npm run lint` (eslint) clean; `npm run typecheck` (tsc) clean; `npm test` (vitest + axe) **9 passed**; `npm run build` (static export) succeeds — shared First-Load JS **103 kB** (< 200 KB budget); `npm audit --omit=dev --audit-level=high` **0 vulnerabilities**; API-client regeneration is drift-free.
- Backend unaffected (no Go changes); the six backend jobs remain green on the PR.

## 6. CI evidence

_(captured on push — see the delivery-review report and the registry row.)_
