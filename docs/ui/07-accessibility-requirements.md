# ForecastIQ — Accessibility Requirements (Reconciled)

**Version**: 1.0 (UI ↔ Backend Reconciliation Board)
**Status**: Authoritative — WCAG 2.1 AA target (DB-08, NFR binding)
**Inputs**: `docs/ui/02-ui-design-specification.md` §14.4; `docs/ui/03-operational-dashboard-design.md` §7; reconciliation board accessibility review

Verdict (Accessibility Lead): **the approved design can meet WCAG 2.1 AA**, conditional on the backend contracts below being honored. No design element depends on inaccessible visual-only communication after the amendments in §4.

---

## 1. Backend-Dependent Accessibility Requirements

Accessible frontend behaviour requires specific backend response properties. These are binding on the API:

| Accessible behaviour | Required backend support | Endpoint(s) |
|----------------------|--------------------------|-------------|
| Chart data available in table form (screen-reader fallback) | Every chart's full data series returned as structured arrays (not pre-rendered); values with exact precision; time points with ISO timestamps | `/forecast-comparison`, `/accuracy` |
| Status not conveyed by colour alone | Machine-readable status strings: `ranking_status`, `freshness.state`, `collection_status`, `circuit_state` — UI renders text badges from these, never colour inference | all |
| Meaningful freshness labels | `freshness{state, last_updated, age_seconds, threshold_seconds}` — enables "Data stale: last updated 4h ago" announcements, not just a dot | all time-sensitive |
| Meaningful confidence labels | `ci_lower`, `ci_upper`, `sample_count` on every metric and ranking — enables "95% CI [0.91, 0.96], 720 samples" tooltips/announcements | `/rankings`, `/accuracy*` |
| Metric definitions reachable | `/rankings/methodology` registry with stable anchors per metric — enables labelled "what does MAE mean?" links | `/rankings/methodology` |
| Localized date/time/numeric formatting | All timestamps ISO 8601 UTC (`Z`); `location.timezone` (IANA) in payloads; units explicit per field naming (`_c`, `_mm`, `_ms`, `_hpa`, `_pct`) — client formats with `Intl` | all |
| Keyboard-operable filters | Filter options returned as enumerable lists (locations, providers, horizons, variables, metric types) — UI renders as focusable controls | `/locations`, `/providers`, `/rankings/methodology` |
| Screen-reader announcements for updates/failures | `warnings[]` with per-provider `message` strings; error envelope with `title` + `detail` — UI feeds `aria-live` regions from these texts | all |
| Accessible error summaries | `errors[{field, message}]` array on 422 — UI renders focusable, labelled field errors | mutations |
| Non-colour provider identification | `provider.id` + `provider.name` on every series — legend text labels; dash patterns available as secondary channel | all chart data |

## 2. Per-Screen Accessibility Contract

| Screen | Critical requirements |
|--------|----------------------|
| S-01 | Semantic `<table>` with `<th scope="col">`; status badges carry text ("Ranked"/"Provisional"/"Insufficient data"); breakdown expansion `aria-expanded` + `role="region"`; freshness dot `aria-label="Data freshness: {state}"`; Tab through rows, Enter/Space expands |
| S-02 | Each variable section `<section aria-labelledby>`; `scope="row"` on provider column; imbalance warning `role="note"`; null "—" has accessible tooltip (focus-triggerable, not hover-only) |
| S-03 | Grid = semantic table with row + column headers; cell values always text (colour is redundant); tooltips keyboard-accessible via focus |
| S-04 | Chart `role="img"` + `aria-label` summary; hidden data table with all bucket values; legend toggles `role="button"` + `aria-pressed`; arrow-key data-point navigation with `aria-live="polite"` tooltip announcement; trend direction as text (↑/↓) not colour; aggregation selector `role="radiogroup"` |
| S-05 | Same chart pattern as S-04; observation gaps announced ("No observation for this hour"); provenance badge text ("Reanalysis") not colour-only; issued-at subtitle in text |
| S-06 | Heading hierarchy for anchor navigation; formulas have plain-language statements alongside math (cognitive accessibility) |
| S-09 | All inputs with visible `<label>`; key-display dialog traps focus; copy button confirms via `aria-live` ("Key copied") |
| S-10 | Freshness text label + dot; circuit state bold text "OPEN"; retry button specific `aria-label` ("Re-collect now for OpenWeather at Johor Bahru"); volume bar `role="progressbar"` + `aria-valuenow`; auto-refresh changes announced `aria-live="polite"` |
| S-11–S-14 | Forms labelled; confirmation dialogs focus-trapped; destructive actions require typed confirmation (motor-error guard); audit table semantic |
| S-15 | Error panels `role="alert"`; focus moves to primary action (Retry/Sign in) |
| Global | Visible focus (2px primary outline, 2px offset); skip-to-content link; logical heading order; `prefers-reduced-motion` disables skeleton pulse + transitions; touch targets ≥ 44×44px |

## 3. Colour Independence Audit (ratified from doc 03 §7.3)

| Information | Colour channel | Redundant channel (binding) |
|-------------|---------------|-----------------------------|
| Observed vs. forecast | Gray-900 vs. provider hue | Line weight/style (solid 2.5px vs. 1.5–2px) + legend text labels ("Observed"/"Forecast") |
| Freshness | Green/amber/orange/red | Text label always rendered alongside dot |
| Ranking status | Green/amber/gray | Badge text always present |
| Provider identity | Unique hue | Legend text + optional dash patterns |
| Impact/severity (admin errors) | Red/amber | Error code text + message |
| Best value in tables | Green underline | Bold weight (secondary cue) |

## 4. Amendments from Accessibility Review

| # | Finding | Resolution |
|---|---------|------------|
| A-01 | Doc 03 §1.1 used emoji (📍, 🔔) as functional indicators | Emoji are decorative only in MVP; functional indicators are Lucide SVG icons with `aria-label`. Emoji in design docs are placeholders. |
| A-02 | Doc 03 freshness dot in Location Context Bar could render colour-only | Binding: dot always paired with text label (doc 02 pattern governs). |
| A-03 | Chart hover tooltips are mouse-dependent | Binding: keyboard arrow navigation + `aria-live` announcement (doc 02 §5.5 governs; applies to all charts). |
| A-04 | "Hollow points" for provisional data are shape-only at small sizes | Binding: legend entry "Provisional (< n samples)" + tooltip sample count; hollow shape is one of three channels (colour, shape, legend text). |
| A-05 | Numeric density (mono 14px) may challenge low-vision users | Acceptable: WCAG requires 4.5:1 contrast (met: #111827 on #FFFFFF = 17.4:1); browser zoom supported (no fixed-height clipping; responsive reflow to 400% per WCAG 1.4.10). |
| A-06 | Auto-refresh on S-10 could disorient screen-reader users | Binding: state changes announced via `aria-live="polite"` (not assertive); refresh does not move focus. |

## 5. Testing Requirements (Accessibility)

- Automated: axe-core in CI on all screens (zero critical/serious violations).
- Component: Testing Library queries enforce labelled controls (tests fail on unlabelled inputs).
- Manual: keyboard-only pass per screen (checklist in `docs/testing/01-requirement-test-traceability.md`); screen-reader pass (VoiceOver + NVDA) on S-01, S-04, S-05 before launch.
- Contrast: all token pairs verified ≥ 4.5:1 (text) / ≥ 3:1 (UI components) — verified values in doc 02 §1.3; any token change requires re-verification.
