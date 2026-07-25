# WP-21 — Core MVP Screens: Implementation Report

**Work package**: WP-21 — Core MVP Screens (15 screens, all 19 states)
**Branch**: `feature/wp21-*` (slices 1–7, merged sequentially to main)
**Final main SHA**: TBD (after slice 7 merge)

---

## Delivery Summary

WP-21 delivers all 14 dashboard screens (S-01 through S-14) plus the S-15 error boundary (delivered in WP-20) with polish refinements, Playwright e2e critical-flow tests, and full CI integration. The screens are implemented as a Next.js 15 static export (`output: 'export'`), deployed via CDN (Cloudflare Pages with SPA fallback).

## Slices Delivered

| Slice | Screens | PR | Key Components |
|-------|---------|-----|----------------|
| 1 | S-01 Overview + S-02 Location Detail | #21 | RankingTable, MetricTable, StatusBadge, SkeletonBlock, useApi (SWR), useGlobalParams, LocationSelector, HorizonSelector |
| 2 | S-03 Provider Detail + S-04 Trends | #22 | Recharts 2.15.0, TrendChart (CI bands, gaps, hollow dots, keyboard nav, hidden data table), ProviderGrid, VariableSelector, AggregationSelector, DateRangePicker, ExportButton |
| 3 | S-05 Forecast vs. Actual + S-06 Methodology | #23 | OverlayChart (obs dashed + forecast solid + error band), DayMetricsTable, Methodology page (formula + weights + thresholds) |
| 4 | S-07 Onboarding + S-09 Settings | #24 | OnboardingDialog (focus-trapped, localStorage), ConfirmDialog (typed confirmation), Settings tabs (profile/keys/preferences/danger zone) |
| 5 | S-10 Admin Health + S-11 Admin Providers | #25 | Admin layout (role guard), HealthGrid (60s auto-refresh, retry), ProviderAdminTable (enable/disable, config edit) |
| 6 | S-12 Admin Locations + S-13 Schedules + S-14 Users | #26 | LocationAdminTable (CRUD, proximity 409), ScheduleTable (pause/resume), UserAdminTable (role/status/export) |
| 7 | S-15 polish + Playwright e2e | #TBD | Playwright critical flows (6 tests), bundle size CI guard, viewport meta, favicon |

## Test Counts

- **Unit/component (vitest + axe-core)**: 42 tests
- **E2E (Playwright)**: 6 critical-flow tests
- **Total**: 48 tests

## Bundle & Performance

- Shared First-Load JS: 103 kB (< 200 kB budget)
- Full static export: ~2.1 MB (< 4 MB guard)
- npm audit: 0 vulnerabilities (production deps)
- Recharts: ~45 kB gzip (within 200 kB chart-lib budget)

## Architecture Decisions

- **Recharts** selected over Visx/Chart.js for CI band support, null-gap handling, custom dots, and React-native integration.
- **SWR** (9 kB) for data fetching: revalidateOnFocus off, refreshInterval for admin health (60s).
- **Static export + SPA fallback**: generateStaticParams placeholder for dynamic routes; Cloudflare Pages rewrites handle runtime routing.
- **Admin layout role guard**: client-side GET /me check; 401 redirects, non-admin renders 403 ErrorPanel.
- **Typed confirmation (ConfirmDialog)**: motor-error guard for destructive actions per doc 02 §14.3 / SEC-10.

## Accessibility

- axe-core CI on all components (42 tests, 0 violations)
- Semantic HTML: tables with `<th scope="col">`, expandable rows via `<button aria-expanded>`, dialogs with `role="dialog"` / `role="alertdialog"` + aria-modal + focus trap
- Charts: `role="img"` + aria-label, hidden data table for SR, keyboard arrow nav + aria-live announcements, focusable legend toggles
- Skip link, visible focus (`:focus-visible`), `prefers-reduced-motion`, non-color status channels (text always present)
- WCAG 2.1 AA foundations across all 14 screens

## Deviations from Original Plan

- **Next 15 / React 19** (forced by security: Next 14.x high advisories only patched in 15.5.x)
- **Bundle 2.1 MB** (full export with Recharts + 14 screens; shared JS remains within 103 kB budget)
- **Responsive card layout deferred**: mobile tables scroll horizontally; full card transformation is a follow-on polish item
