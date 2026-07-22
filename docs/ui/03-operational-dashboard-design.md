# ForecastIQ — Operational Overview Dashboard Design

**Version**: 1.0
**Status**: Design Specification — Production-Quality Desktop SaaS
**Source of Truth**: All Phase 0 Amendment authoritative documents
**Target Viewport**: 1440 × 1024 px (desktop)
**Location Context**: Johor Bahru, Malaysia · 21 July 2026

---

## 1. High-Fidelity Dashboard Layout

### 1.1 Application Shell (1440 × 1024)

```
┌──────────────────────────────────────────────────────────────────────────────────────┐
│ SIDEBAR (240px)  │  MAIN CONTENT AREA (1200px)                                       │
│                  │                                                                     │
│ ┌──────────────┐ │  TOP BAR (56px height)                                            │
│ │ ◈ ForecastIQ │ │  ┌─────────────────────────────────────────────────────────────┐  │
│ └──────────────┘ │  │ [📍 Johor Bahru ▾]  [Mon, Jul 21 · MYT UTC+8]  [⌘K Search]│  │
│                  │  │                              [🔔 2] [● AS]                   │  │
│ OVERVIEW         │  └─────────────────────────────────────────────────────────────┘  │
│  ◉ Overview     │                                                                     │
│  ○ Live Weather │  ┌─────────────────────────────────────────────────────────────┐  │
│  ○ Forecasts    │  │                                                               │  │
│  ○ Accuracy     │  │  LOCATION CONTEXT BAR                                         │  │
│  ○ Provider     │  │  Johor Bahru, Johor · MYT (UTC+8)                            │  │
│    Comparison   │  │  Last observation: Jul 21, 18:05 MYT (12 min ago)            │  │
│  ○ Locations    │  │  ● Fresh  ·  Source: Open-Meteo Historical [Reanalysis]      │  │
│  ○ Alerts       │  │  [Switch location ↗]                                          │  │
│  ○ Reports      │  │                                                               │  │
│                  │  ├───────────────────────────────────────────────────────────────┤  │
│ DATA OPERATIONS  │  │                                                               │  │
│  ○ Forecast Runs│  │  CURRENT STATUS (Observed)    FORECAST CONSENSUS (Predicted)  │  │
│  ○ Observations │  │  ┌─────────────────────────┐  ┌─────────────────────────────┐│  │
│  ○ Collection   │  │  │  OBSERVED               │  │  FORECAST CONSENSUS          ││  │
│    Health       │  │  │  ─────────────────────── │  │  ─────────────────────────── ││  │
│                  │  │  │  Temperature   31.4 °C  │  │  Predicted temp    30.8 °C   ││  │
│ ADMINISTRATION   │  │  │  Feels like    36.2 °C  │  │  Rain probability  72%       ││  │
│  ○ Providers    │  │  │  Rainfall      0.0 mm   │  │  Expected rainfall 4.8 mm    ││  │
│  ○ Integrations │  │  │  Humidity      74%      │  │  Confidence        Moderate  ││  │
│  ○ API Keys     │  │  │  Wind          2.1 m/s  │  │  Provider agreement 3/4      ││  │
│  ○ Team         │  │  │  Wind dir      220° SW  │  │  Disagreement      Evening   ││  │
│  ○ Settings     │  │  │                         │  │                               ││  │
│                  │  │  │  Source: Open-Meteo     │  │  Next 24h · +24h horizon     ││  │
│ ─────────────── │  │  │  Historical (reanalysis)│  │  Methodology v2026.1         ││  │
│ v1.0.0 · ● Live │  │  └─────────────────────────┘  └─────────────────────────────┘│  │
│                  │  │                                                               │  │
│                  │  ├───────────────────────────────────────────────────────────────┤  │
│                  │  │                                                               │  │
│                  │  │  PRIMARY CHART: Temperature — Observed vs Forecast            │  │
│                  │  │  ┌─────────────────────────────────────────────────────────┐  │  │
│                  │  │  │  °C                                                      │  │  │
│                  │  │  │  34 ┤                                                    │  │  │
│                  │  │  │  33 ┤        ╭──╮                    ╭───                │  │  │
│                  │  │  │  32 ┤   ╭───╯  ╰──╮    ╭──╮  ╭───╯                    │  │  │
│                  │  │  │  31 ┤───╯         ╰──╮╭╯  ╰──╯                        │  │  │
│                  │  │  │  30 ┤                ╰╯         ╰───                    │  │  │
│                  │  │  │  29 ┤                                                   │  │  │
│                  │  │  │  28 ┤───────────────────────────────────────────────────│  │  │
│                  │  │  │  27 ┤                                                   │  │  │
│                  │  │  │  26 ┤                                                   │  │  │
│                  │  │  │  25 ┤                                                   │  │  │
│                  │  │  │     └──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──┬──  │  │  │
│                  │  │  │      06 08 10 12 14 16 18 20 22 00 02 04 06 08 10 12   │  │  │
│                  │  │  │                    ▲ NOW (18:17 MYT)                     │  │  │
│                  │  │  │                                                          │  │  │
│                  │  │  │  ━━━ Observed (actual)    ─ ─ Open-Meteo forecast       │  │  │
│                  │  │  │  ━━━ Tomorrow.io          ─ ─ Visual Crossing           │  │  │
│                  │  │  │  ░░░ Confidence band (±MAE) ─ ─ OpenWeather             │  │  │
│                  │  │  └─────────────────────────────────────────────────────────┘  │  │
│                  │  │  Legend: [━ Observed] [─ Open-Meteo] [─ Tomorrow.io]          │  │
│                  │  │          [─ Visual Crossing] [─ OpenWeather] [░ ±MAE band]    │  │
│                  │  │  "Forecasts issued Jul 21, 17:00 MYT · Horizon +1h to +24h"  │  │
│                  │  │  Timezone: MYT (UTC+8) · Source: Open-Meteo Historical       │  │
│                  │  │                                                               │  │
│                  │  ├───────────────────────────────────────────────────────────────┤  │
│                  │  │                                                               │  │
│                  │  │  ┌──────────────────────────┐ ┌────────────────────────────┐ │  │
│                  │  │  │ PROVIDER RELIABILITY      │ │ FORECAST CHANGES           │ │  │
│                  │  │  │ ───────────────────────── │ │ ────────────────────────── │ │  │
│                  │  │  │ #1 Open-Meteo     0.88   │ │ ▲ Rain prob 44% → 72%     │ │  │
│                  │  │  │ #2 Tomorrow.io    0.84   │ │   High impact · 14:32 MYT  │ │  │
│                  │  │  │ #3 Visual Crossing 0.81  │ │                            │ │  │
│                  │  │  │ #4 OpenWeather    0.77   │ │ ▲ Rain start 17:00 → 15:30│ │  │
│                  │  │  │                          │ │   Medium impact · 13:15    │ │  │
│                  │  │  │ 30-day period · JB       │ │                            │ │  │
│                  │  │  │ 720 samples · +24h       │ │ ▼ Tomorrow.io −3.2 mm     │ │  │
│                  │  │  │ Methodology v2026.1      │ │   Low impact · 12:00      │ │  │
│                  │  │  │ [Full accuracy →]        │ │                            │ │  │
│                  │  │  └──────────────────────────┘ │ ▲ Disagreement ↑ evening  │ │  │
│                  │  │                               │   Medium impact · 16:45    │ │  │
│                  │  │                               │ [View all changes →]       │ │  │
│                  │  │                               └────────────────────────────┘ │  │
│                  │  │                                                               │  │
│                  │  ├───────────────────────────────────────────────────────────────┤  │
│                  │  │  OPERATIONAL HEALTH (compact strip)                           │  │
│                  │  │  ● 4/4 collectors healthy · Obs: 12m ago · API p50: 340ms   │  │
│                  │  │  Last collection: Jul 21, 18:00 MYT · Retries: 0 · No gaps   │  │
│                  │  │                                                               │  │
│                  │  ├───────────────────────────────────────────────────────────────┤  │
│                  │  │  ATTRIBUTION FOOTER                                           │  │
│                  │  │  Data: Open-Meteo (link) · OpenWeather (link) · Tomorrow.io  │  │
│                  │  │  (link) · Visual Crossing (link) · Obs: Open-Meteo Hist.     │  │
│                  │  │  Methodology v2026.1 · ForecastIQ measures forecasts.        │  │
│                  │  └─────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────────────────────┘
```

### 1.2 Visual Hierarchy Zones (top → bottom)

| Zone | Height | Purpose | Visual Weight |
|------|--------|---------|---------------|
| Top Bar | 56px | Global controls, context | Minimal — flat, border-bottom only |
| Location Context | 48px | Where + when + freshness | Text-only, no container |
| Observed + Forecast | 168px | Current status at a glance | Two equal panels, labelled, bordered |
| Primary Chart | 360px | Temperature comparison (dominant) | Largest element; card with subtle border |
| Provider + Changes | 240px | Reliability ranking + forecast delta | Two-column, equal width |
| Operational Health | 44px | Pipeline status strip | Single-line, minimal |
| Attribution Footer | 48px | Legal + provenance | Smallest text, muted |

### 1.3 Sidebar Specification

**Width**: 240px expanded / 64px collapsed
**Background**: `#FFFFFF` with 1px right border `--color-border`
**Logo area**: 56px height, aligned with top bar

| Section | Items | Icon (Lucide) |
|---------|-------|---------------|
| (ungrouped) | Overview | LayoutDashboard |
| | Live Weather | CloudSun |
| | Forecasts | CalendarClock |
| | Accuracy | Target |
| | Provider Comparison | GitCompare |
| | Locations | MapPin |
| | Alerts | Bell |
| | Reports | FileBarChart |
| Data Operations | Forecast Runs | RefreshCw |
| | Observations | Radio |
| | Collection Health | Activity |
| Administration | Providers | Server |
| | Integrations | Plug |
| | API Keys | Key |
| | Team | Users |
| | Settings | Settings |

**Active state**: 2px left border `--color-primary` + `#EFF6FF` (blue-50) background + primary text color
**Hover**: `--color-surface-secondary` background
**Section labels**: 11px uppercase, `--color-text-muted`, 8px letter-spacing
**Collapsed state**: Icons only (24px), tooltip on hover, 64px width

**Footer**: Version indicator "v1.0.0" + system status dot (green = operational)

### 1.4 Top Bar Specification

**Height**: 56px
**Background**: `#FFFFFF`
**Border**: 1px bottom `--color-border`
**Content (left → right)**:

| Element | Spec |
|---------|------|
| Location selector | Dropdown with search; shows "📍 Johor Bahru" + chevron; 200px width |
| Date/timezone context | "Mon, Jul 21, 2026 · MYT (UTC+8)" — 13px secondary text |
| Command/search | "⌘K" pill input, 240px, placeholder "Search or jump to…"; centered |
| Notifications | Bell icon + count badge "2" (amber); 36px target |
| User profile | 32px avatar circle "AS" + chevron; dropdown: Profile, Settings, Sign out |

---

## 2. Information Hierarchy

### 2.1 Design Rationale

The dashboard answers six questions in descending priority:

| Priority | Question | Zone | Treatment |
|----------|----------|------|-----------|
| 1 | What is happening now? | Observed panel | Immediate, factual, labelled "OBSERVED" |
| 2 | What is expected next 24h? | Forecast panel + Chart | Predictive, labelled "FORECAST", visually distinct |
| 3 | How confident should I be? | Confidence rating + agreement + CI band | Embedded in forecast panel and chart |
| 4 | Which provider is most reliable? | Provider Reliability | Ranked list with full methodology context |
| 5 | Has the forecast changed? | Forecast Changes feed | Timestamped deltas with impact levels |
| 6 | Is data collection normal? | Operational Health strip | Single-line ambient awareness |

### 2.2 Observed vs. Forecast Distinction (Critical)

Per Phase 0 Product Contract NP-01 and the data model's strict separation of `Observation` and `ForecastSnapshot` entities:

| Aspect | Observed | Forecast |
|--------|----------|----------|
| Panel label | "OBSERVED" (uppercase, 11px, tracking) | "FORECAST CONSENSUS" (uppercase) |
| Left border accent | 3px solid `#111827` (gray-900) | 3px solid `#1A56DB` (primary blue) |
| Icon | Filled circle (●) | Hollow circle (○) |
| Chart line style | Solid, 2.5px, gray-900 | Dashed or thinner solid, provider-colored |
| Provenance | Always shows source + observation_type badge | Shows issued_at + horizon |
| Language | "Actual", "Observed", "Measured" | "Predicted", "Expected", "Forecast" |

This distinction is non-negotiable. Users must never confuse observations with predictions.

### 2.3 Progressive Disclosure

- **Glanceable** (no interaction): current temp, rain probability, freshness, #1 provider
- **One interaction**: chart hover → exact values; click provider → detail; click change → context
- **Drill-down**: "Full accuracy →" navigates to Accuracy Analytics; "View all changes →" to Forecast Evolution
- **Hidden until needed**: methodology details behind ⓘ links; API details behind admin nav

---

## 3. Reusable Components

| Component | Description | Usage |
|-----------|-------------|-------|
| `AppSidebar` | Persistent left nav with sections, collapse toggle, active states | Shell |
| `TopBar` | Global controls: location, date/tz, search, notifications, profile | Shell |
| `LocationContextBar` | Location name, timezone, last observation, freshness dot, provenance badge | Overview, Live Weather |
| `ObservedPanel` | Current observation metrics grid with source attribution | Overview |
| `ForecastConsensusPanel` | Predicted metrics with confidence, agreement, horizon context | Overview |
| `TemperatureComparisonChart` | Multi-series time chart with observed line, provider forecasts, CI band, now marker | Overview (primary) |
| `ChartLegend` | Keyboard-accessible series toggles with pattern/color/label | All charts |
| `ProviderReliabilityList` | Ranked provider list with score, period, sample context | Overview, Accuracy |
| `ForecastChangeFeed` | Timestamped forecast deltas with impact badges | Overview, Forecast Evolution |
| `OperationalHealthStrip` | Single-line pipeline status summary | Overview (compact) |
| `FreshnessIndicator` | Dot + label (fresh/delayed/stale/unavailable) per BR-FRESH | All data views |
| `ProvenanceBadge` | Observation type badge (Station/Reanalysis/Interpolated/Provider est.) | Observation displays |
| `StatusBadge` | Ranked/Provisional/Unranked with text + color | Rankings |
| `ImpactBadge` | High/Medium/Low with icon + color (non-color-only) | Forecast changes |
| `MetricValue` | Tabular-numeral value + unit + optional CI | All metrics |
| `AttributionFooter` | Provider attribution links + methodology version + disclaimer | All data pages |
| `CommandPalette` | ⌘K search overlay for navigation and quick actions | Global |
| `NotificationDropdown` | Alert list with acknowledge action | Top bar |
| `SkeletonBlock` | Loading placeholder (card/chart/table/text variants) | All screens |
| `ErrorPanel` | Error state with retry, cached data label | All screens |
| `EmptyState` | Variants: no-location, no-data, insufficient, observation-unavailable | All screens |
| `StaleBanner` | Orange banner with last-updated time per BR-FRESH-01 | All data screens |

---

## 4. Design Tokens

### 4.1 Colour

#### Semantic Palette

| Token | Value | Usage |
|-------|-------|-------|
| `--color-primary` | `#1A56DB` | Primary actions, active nav, links, forecast accent |
| `--color-primary-hover` | `#1444B0` | Hover state |
| `--color-primary-subtle` | `#EFF6FF` | Active nav background, selected states |
| `--color-surface` | `#FFFFFF` | Cards, panels, sidebar, top bar |
| `--color-surface-secondary` | `#F9FAFB` | Page background, alternating rows |
| `--color-border` | `#E5E7EB` | Card borders, dividers, separators |
| `--color-border-strong` | `#D1D5DB` | Input borders, emphasized dividers |
| `--color-text-primary` | `#111827` | Headings, primary content, observed data |
| `--color-text-secondary` | `#6B7280` | Captions, metadata, secondary labels |
| `--color-text-muted` | `#9CA3AF` | Disabled, placeholder, section labels |

#### Status Colours

| Token | Value | Usage |
|-------|-------|-------|
| `--color-positive` | `#059669` | Fresh data, ranked status, positive trend |
| `--color-warning` | `#D97706` | Delayed, provisional, medium impact, uncertain |
| `--color-severe` | `#DC2626` | Unavailable, error, high impact, poor performance |
| `--color-info` | `#2563EB` | Informational badges, low impact |

#### Provider Colours (consistent across all charts)

| Provider | Colour | Dash Pattern |
|----------|--------|--------------|
| Observed (actual) | `#111827` | Solid, 2.5px |
| Open-Meteo | `#2563EB` (blue-600) | Solid, 1.5px |
| Tomorrow.io | `#0891B2` (cyan-600) | Solid, 1.5px |
| Visual Crossing | `#7C3AED` (violet-600) | Solid, 1.5px |
| OpenWeather | `#DB2777` (pink-600) | Solid, 1.5px |
| Confidence band | Provider colour at 8% opacity | Filled area |

#### Chart-Specific

| Token | Value | Usage |
|-------|-------|-------|
| `--color-now-marker` | `#DC2626` | Current-time vertical line |
| `--color-grid-line` | `#F3F4F6` | Chart gridlines |
| `--color-axis-text` | `#6B7280` | Axis labels |

### 4.2 Typography

**Primary typeface**: Inter (all UI text)
**Monospace**: JetBrains Mono (numeric values, scores, timestamps)

| Token | Size | Weight | Line Height | Usage |
|-------|------|--------|-------------|-------|
| `--text-page-title` | 28px | 700 | 36px | Page titles |
| `--text-section-heading` | 20px | 600 | 28px | Section headers |
| `--text-card-title` | 16px | 600 | 24px | Panel/card titles |
| `--text-body` | 14px | 400 | 20px | Default text |
| `--text-body-small` | 13px | 400 | 18px | Secondary text, captions |
| `--text-metric-value` | 24px (mono) | 600 | 32px | Large metric numbers |
| `--text-metric-inline` | 14px (mono) | 500 | 20px | Inline numeric values |
| `--text-label` | 12px | 500 | 16px | Badges, axis labels, section labels |
| `--text-label-uppercase` | 11px | 600 | 16px | Uppercase tracking labels (OBSERVED, FORECAST) |
| `--text-micro` | 11px | 400 | 14px | Attribution footer, legal |

**Tabular numerals**: `font-variant-numeric: tabular-nums` on all metric values, scores, timestamps.

### 4.3 Spacing

Base unit: 8px. Page gutters: 24px.

| Token | Value | Usage |
|-------|-------|-------|
| `--space-1` | 4px | Icon-to-text gap |
| `--space-2` | 8px | Intra-component padding, badge padding |
| `--space-3` | 12px | Input padding, list item gap |
| `--space-4` | 16px | Card internal padding, between related elements |
| `--space-5` | 20px | Panel padding |
| `--space-6` | 24px | Page gutters, between cards |
| `--space-8` | 32px | Section separators |
| `--space-10` | 40px | Major section gaps |
| `--space-12` | 48px | Page-level vertical rhythm |

### 4.4 Border Radius

| Token | Value | Usage |
|-------|-------|-------|
| `--radius-sm` | 4px | Badges, tags, small buttons |
| `--radius-md` | 6px | Inputs, buttons, dropdowns |
| `--radius-lg` | 8px | Cards, panels, chart containers |
| `--radius-full` | 9999px | Avatar, dots, pills |

### 4.5 Elevation

| Token | Value | Usage |
|-------|-------|-------|
| `--shadow-none` | none | Default cards (border-only separation) |
| `--shadow-sm` | `0 1px 2px rgba(0,0,0,0.05)` | Dropdowns, popovers |
| `--shadow-md` | `0 4px 6px -1px rgba(0,0,0,0.1)` | Modals, command palette |
| `--shadow-lg` | `0 10px 15px -3px rgba(0,0,0,0.1)` | Slide-over drawers |

**Principle**: Use borders (`1px --color-border`) for static containers. Shadows only for floating/overlay elements. No cards inside cards.

---

## 5. Interaction Notes

### 5.1 Primary Interactions

| Interaction | Trigger | Behaviour |
|-------------|---------|-----------|
| Switch location | Click location selector in top bar | Dropdown with search; URL updates `?location_id=`; all panels refetch with skeleton |
| Change date range | Date context click → date picker | URL param update; chart + panels refetch |
| Change weather variable | Variable tabs above chart (Temperature / Precipitation / Wind / Humidity) | Chart y-axis + series update; observed/forecast panels update relevant metrics |
| Inspect provider details | Click provider name in reliability list | Right slide-over drawer: full metric breakdown, per-horizon scores, collection health |
| Open forecast-change details | Click a change feed item | Expand inline: shows previous value, new value, affected providers, time delta |
| Navigate to full accuracy | Click "Full accuracy →" link | Route to `/accuracy?location_id=&horizon_minutes=` |
| View map | Click "Map" toggle in chart header or Live Weather nav | Navigate to `/live-weather` with map view active |
| Acknowledge alert | Click notification bell → click "Acknowledge" on an item | Badge count decrements; item marked read; `aria-live` announcement |
| Chart hover | Mouse move over chart area | Vertical crosshair + tooltip: time, observed value, each provider's forecast, error |
| Chart keyboard nav | Arrow keys when chart focused | Move crosshair left/right; tooltip announced via `aria-live="polite"` |
| Toggle chart series | Click legend item | Series hidden/shown; legend item gets strikethrough; `aria-pressed` state |
| Command palette | `⌘K` or `Ctrl+K` | Overlay: search locations, providers, screens; recent items; quick actions |
| Sidebar collapse | Click collapse toggle (chevron) | Sidebar animates to 64px; icons only; tooltips on hover |

### 5.2 Drawer Pattern (Provider Detail)

- Slides from right, 400px width
- Contains: provider name, composite score, per-metric breakdown table, collection health summary, link to full provider page
- Closes on Escape, outside click, or close button
- Focus trapped while open

### 5.3 Tooltip Behaviour

- Delay: 300ms hover, immediate on focus
- Position: above element, centered; flips if viewport edge
- Content: concise text + optional metadata (e.g., CI explanation)
- Dismissal: mouse leave, Escape, scroll
- Min target: tooltip trigger ≥ 24px clickable area

---

## 6. Responsive Behaviour

### 6.1 Breakpoints

| Token | Width | Strategy |
|-------|-------|----------|
| `desktop` | > 1280px | Full layout as specified |
| `tablet` | 768–1280px | Sidebar collapsed (64px); panels stack; chart full-width |
| `mobile` | < 768px | Sidebar hidden (hamburger); single column; simplified views |

### 6.2 Tablet (768–1280px)

- Sidebar: auto-collapsed to 64px icons
- Observed + Forecast panels: stack vertically (full width each)
- Chart: full-width, 300px height
- Provider Reliability + Forecast Changes: stack vertically
- Operational Health: unchanged (single line)
- Top bar: location selector + avatar; date moves below; search icon-only (expands on tap)

### 6.3 Mobile (< 768px)

- **Not the full analytics experience.** Focus: current conditions + alert review.
- Sidebar: hidden; hamburger opens full-screen nav drawer
- Top bar: location name + avatar only
- Observed panel: full-width card, key metrics (temp, rain, humidity)
- Forecast panel: full-width card, key predictions
- Chart: full-width, 240px height, simplified legend (horizontal scroll chips)
- Provider reliability: top-2 only with "See all →" link
- Forecast changes: latest 2 items with "See all →"
- Operational health: hidden (accessible via nav → Collection Health)

### 6.4 Touch Targets

- Minimum: 44 × 44px (WCAG 2.5.5)
- Table rows (mobile): 48px min height
- Chart legend items: 36px height + 8px gap
- Selector chips: 36px height

---

## 7. Accessibility Considerations

### 7.1 WCAG 2.1 AA Compliance

| Requirement | Implementation |
|-------------|----------------|
| Contrast ≥ 4.5:1 (text) | All text tokens verified against backgrounds; `--color-text-secondary` (#6B7280) on white = 4.6:1 ✓ |
| Contrast ≥ 3:1 (UI/large text) | Chart lines, badges, borders meet 3:1 against white |
| Keyboard navigation | All interactive elements focusable; logical Tab order; Enter/Space activates |
| Visible focus | 2px outline `--color-primary` + 2px offset on all focusable elements |
| Non-colour status | Freshness: dot + text label; Impact: icon + text; Status: badge text |
| Chart accessibility | `role="img"` + `aria-label` summary; hidden data table for screen readers |
| Chart keyboard | Arrow keys navigate data points; tooltip content in `aria-live="polite"` region |
| Legend toggles | `role="button"` + `aria-pressed`; series identified by label, not color alone |
| Line differentiation | Observed = solid thick; forecasts = thinner; patterns (dash) available as option |
| Reduced motion | `prefers-reduced-motion`: disables skeleton pulse, transitions, chart animations |
| Form labels | All inputs have visible `<label>`; selects have `aria-label` |
| Error announcements | `role="alert"` on error panels; focus moves to retry action |
| Loading state | `aria-busy="true"` on loading regions |
| Semantic tables | `<table>`, `<th scope="col">`, `<th scope="row">` for all data tables |

### 7.2 Screen Reader Chart Alternative

Every chart includes a visually-hidden `<table>` containing:
- Column headers: Time, Observed, Open-Meteo, Tomorrow.io, Visual Crossing, OpenWeather
- One row per time point with exact values
- Accessible via screen reader table navigation

### 7.3 Colour Independence

| Information | Colour Channel | Redundant Channel |
|-------------|---------------|-------------------|
| Observed vs Forecast | Gray-900 vs colored | Solid thick vs thinner; label "OBSERVED" / "FORECAST" |
| Freshness state | Green/Amber/Orange/Red | Text label ("Fresh", "Delayed", "Stale") |
| Impact level | Red/Amber/Blue | Icon (▲/▼) + text ("High", "Medium", "Low") |
| Provider identity | Unique hue | Legend text label; different dash patterns available |
| Ranking status | Green/Amber/Gray | Badge text ("Ranked", "Provisional", "Insufficient data") |

---

## 8. States

### 8.1 Healthy State (Default — Shown Above)

All data fresh, all collectors operational, all providers reporting. The design in §1.1 represents the healthy state.

### 8.2 Loading State

| Zone | Skeleton |
|------|----------|
| Location Context | Text bar (200px × 16px) + dot + badge shape |
| Observed Panel | 6 horizontal bars (varying widths) |
| Forecast Panel | 6 horizontal bars |
| Chart | Full rectangle (360px) with subtle pulse |
| Provider Reliability | 4 rows of bars |
| Forecast Changes | 4 rows of bars |
| Operational Health | Single bar |

**Animation**: Opacity pulse 0.4 → 1.0, 1.5s ease-in-out loop. Disabled under `prefers-reduced-motion`.
**Refetch**: Previous data shown at 50% opacity with skeleton overlay. No layout shift.
**Debounce**: Skeleton only shown if fetch > 100ms.

### 8.3 Empty State — No Locations

- Centered in main content area
- Icon: MapPin with dashed circle (48px, `--color-text-muted`)
- Heading: "No locations monitored yet"
- Body: "Locations are added by the platform operator."
- Admin CTA: [Add Location] primary button (admin only)
- No chart axes, no broken panels

### 8.4 Empty State — Location Has No Data

- Icon: Clock (48px)
- Heading: "Collecting data for Johor Bahru"
- Body: "Collecting since Jul 21, 2026 — first observations appear within ~1 hour. Provider rankings require ≥7 days of matched data."
- Progress hint: "0 hours collected / 168 hours for provisional ranking"
- Chart area: flat placeholder with message; no axes

### 8.5 Partial Data State

- Affected provider: amber left border on its legend entry + "Temporarily unavailable" badge
- Unaffected providers render normally
- Subtle note above chart: "OpenWeather data temporarily unavailable — showing 3 of 4 providers"
- Provider reliability: affected row shows "—" score + "Unavailable" badge; rank positions unchanged
- Per Phase 0 screen inventory: unaffected providers always render normally

### 8.6 Stale Data State

- Orange banner (full-width, above Location Context):
  "⚠ Data may be out of date — last updated 4h ago (Jul 21, 14:00 MYT)"
- Freshness dot: orange
- Provider reliability: "stale" badge next to affected scores
- Banner is persistent (not dismissible) while data remains stale
- Per BR-FRESH-01: stale data always shown WITH its staleness

### 8.7 Error State

- Centered error panel in main content:
  - AlertTriangle icon (32px, `--color-severe`)
  - "Unable to load data"
  - "The server may be temporarily unavailable."
  - [Retry] primary button
- If cached data exists: shown dimmed (50% opacity) with "Cached — last loaded 2h ago (Jul 21, 16:00 MYT)" label above error panel
- After 3 failed retries: "Still unable to connect. Check your network or try again later." + retry removed
- `role="alert"`; focus moves to Retry button

### 8.8 Observation Unavailable State

- Observed panel replaced with info panel:
  "Ground truth unavailable for this period — accuracy metrics not computed."
  Provenance note: "Observation source: Open-Meteo Historical (reanalysis blend). Availability varies."
- Chart: forecast lines shown; observed line absent; confidence band absent
- Provider reliability: "Metrics not computed — no observations" note

---

## 9. Screen Inventory — Complete ForecastIQ Product

### 9.1 Primary Screens

| # | Screen | Purpose | Auth |
|---|--------|---------|------|
| S-01 | **Overview Dashboard** | Current status + forecast + trust + reliability + changes + health | Public |
| S-02 | **Live Weather** | Real-time observation detail, radar/map context, observation history | Public |
| S-03 | **Forecasts** | Forecast detail per provider, issued-at timeline, multi-variable view | Public |
| S-04 | **Accuracy Analytics** | Metric trends over time, multi-provider overlay, aggregation | Public |
| S-05 | **Provider Comparison** | Cross-location grid, per-horizon metrics, reliability vs coverage | Public |
| S-06 | **Locations** | Location list, per-location summary, add/edit (admin) | Public/Admin |
| S-07 | **Alerts** | Alert rules, notification history, acknowledge (Level 3) | Signed-in |
| S-08 | **Reports** | Scheduled exports, custom report builder (Level 3) | Signed-in |
| S-09 | **Forecast vs. Actual** | Overlay chart: forecasts vs observation for a date/variable | Public |
| S-10 | **Methodology** | Formulas, weights, thresholds, worked example, version history | Public |

### 9.2 Data Operations Screens

| # | Screen | Purpose | Auth |
|---|--------|---------|------|
| S-11 | **Forecast Runs** | Collection run history, status, replay, recompute | Admin |
| S-12 | **Observations** | Observation log, provenance, quality flags, corrections | Admin |
| S-13 | **Collection Health** | Per provider-location: freshness, circuit state, errors, retry | Admin |

### 9.3 Administration Screens

| # | Screen | Purpose | Auth |
|---|--------|---------|------|
| S-14 | **Providers** | Enable/disable, adapter config, attribution, schedule | Admin |
| S-15 | **Integrations** | Third-party connections, webhook config (Level 3) | Admin |
| S-16 | **API Keys** | Key CRUD, scopes, rate limits, usage | Signed-in |
| S-17 | **Team** | User management, roles, audit log | Admin |
| S-18 | **Settings** | Profile, preferences, timezone, danger zone | Signed-in |

### 9.4 System Screens

| # | Screen | Purpose | Auth |
|---|--------|---------|------|
| S-19 | **Onboarding** | First-use: what we measure, pick location, methodology link | Signed-in, first visit |
| S-20 | **Auth** | Sign in / Sign up / Verify / Reset password | Public |
| S-21 | **Error Pages** | 404, 500, 403, offline | All |

---

## 10. Next Three Screens + Design Proposals

### 10.1 Recommendation: Priority Order

1. **Forecast Evolution** (S-03 Forecasts + S-09 Forecast vs. Actual combined view)
2. **Accuracy Analytics** (S-04)
3. **Live Weather Map** (S-02)

### 10.2 Forecast Evolution — Design Proposal

**Purpose**: Show how forecasts for a target period changed over successive issuances. Answer: "Did the forecast converge or diverge, and when did it shift?"

**Layout**:
- Control bar: Date selector, Variable selector, Target hour range, Provider filter
- Primary chart: X-axis = issuance time (hours before target); Y-axis = forecast value
  - One line per provider showing how their prediction for the target hour evolved
  - Horizontal band = observation ± MAE (the "truth zone")
  - Vertical marker = "forecast frozen" (last issuance before target)
- Secondary chart (below): Provider disagreement index over time (std dev across providers)
- Change log panel (right): Ordered list of material forecast changes with timestamps
- Summary: "Forecast converged at 14:00" or "Disagreement persists for evening period"

**Key interactions**: Brush to select target hour range; click issuance to see full snapshot; toggle providers.

### 10.3 Accuracy Analytics — Design Proposal

**Purpose**: Show how provider accuracy metrics change over time. Answer: "Is this provider improving or degrading?"

**Layout** (per Phase 0 §6, evolved):
- Control bar: Location, Horizon, Variable, Metric type, Period (7d/30d/90d/Custom), Aggregation (daily/weekly/monthly)
- Primary chart: Time series of selected metric per provider
  - Provider-colored lines + CI band (10% opacity)
  - Hollow points where sample_count < threshold (provisional)
  - Y-axis label includes "lower is better" or "higher is better"
- Summary table: Period average, latest value, trend (Δ), samples, CI per provider
- Ranking timeline (secondary): Stacked area or line showing rank changes over time
- Export CSV button with methodology metadata header

**Key interactions**: Click data point → navigate to Forecast vs. Actual for that date; metric selector changes y-axis; aggregation changes bucket size.

### 10.4 Live Weather Map — Design Proposal

**Purpose**: Geographic context for multi-location monitoring. Answer: "What's happening across all my locations right now?"

**Layout**:
- Full-bleed map (left 70%): Tile-based map (CartoDB Positron or similar muted style)
  - Location markers: colored by freshness state (green/amber/red)
  - Marker size: proportional to latest observation significance
  - Hover: popup with location name, temp, rain, freshness
  - Click: navigate to Overview for that location
- Side panel (right 30%): Selected location quick stats
  - Current observation summary
  - Forecast consensus
  - Provider agreement
  - "View dashboard →" link
- Map controls: Zoom, layer toggle (observations / forecast / disagreement heatmap)
- Top: same global controls (location selector syncs with map selection)

**Key constraints**: Map is contextual, not decorative. No animated weather particles. Muted tile palette. Data overlays are the focus.

**Level 3 extension**: Radar overlay, precipitation accumulation heatmap, wind barbs.

---

## Appendix A: Mock Data Reference (Johor Bahru, 21 July 2026)

### Current Observations (18:05 MYT)

| Variable | Value | Source |
|----------|-------|--------|
| Temperature | 31.4 °C | Open-Meteo Historical (reanalysis) |
| Feels like | 36.2 °C | Derived (heat index) |
| Rainfall (last hour) | 0.0 mm | Open-Meteo Historical |
| Humidity | 74% | Open-Meteo Historical |
| Wind speed | 2.1 m/s | Open-Meteo Historical |
| Wind direction | 220° (SW) | Open-Meteo Historical |
| Pressure | 1009 hPa | Open-Meteo Historical |
| Condition | Partly cloudy | Canonical: `partly_cloudy` |

### Forecast Consensus (next 24h, +24h horizon)

| Variable | Consensus | Range | Agreement |
|----------|-----------|-------|-----------|
| Temperature (max) | 30.8 °C | 29.6–32.1 °C | 4/4 providers |
| Rain probability | 72% | 58–85% | 3/4 (OpenWeather dissent: 58%) |
| Expected rainfall | 4.8 mm | 1.6–8.0 mm | Low agreement (evening) |
| Confidence | Moderate | — | Based on provider spread |

### Provider Reliability (30-day, +24h horizon, Johor Bahru)

| Rank | Provider | Composite | Samples | Coverage | Status |
|------|----------|-----------|---------|----------|--------|
| 1 | Open-Meteo | 0.880 | 720 | 98% | Ranked |
| 2 | Tomorrow.io | 0.840 | 680 | 95% | Ranked |
| 3 | Visual Crossing | 0.810 | 650 | 91% | Ranked |
| 4 | OpenWeather | 0.770 | 700 | 92% | Ranked |

Period: Jun 22 – Jul 21, 2026 · Methodology v2026.1 · Weights w-2026.1

### Forecast Changes (Jul 21, 2026)

| Time (MYT) | Change | Impact | Detail |
|------------|--------|--------|--------|
| 14:32 | Rain probability 44% → 72% | High | Consensus shift; 3 providers updated |
| 13:15 | Expected rain start 17:00 → 15:30 | Medium | Earlier onset; Tomorrow.io leading |
| 12:00 | Tomorrow.io rainfall −3.2 mm | Low | Reduced from 8.0 to 4.8 mm |
| 16:45 | Provider disagreement ↑ evening | Medium | Std dev 2.4mm → 4.1mm for 19:00–22:00 |

### Operational Health

| Metric | Value | Status |
|--------|-------|--------|
| Collectors healthy | 4/4 | ● Green |
| Observation freshness | 12 min ago | ● Fresh |
| Provider API latency (p50) | 340 ms | Normal |
| Latest successful collection | Jul 21, 18:00 MYT | On schedule |
| Retry count (24h) | 0 | Normal |
| Missing data (24h) | None | Complete |

---

## Appendix B: Reconciliation Notes with Phase 0

| Aspect | Phase 0 Baseline | This Design | Reconciliation |
|--------|-----------------|-------------|----------------|
| Navigation | Horizontal header nav | Sidebar nav | Sidebar accommodates expanded product scope (Data Ops, Admin sections); Phase 0's public/admin separation preserved |
| Overview focus | Rankings table (S-01) | Operational dashboard | Rankings preserved as "Provider Reliability" section; operational context added per user brief |
| Providers | 2 (Open-Meteo, OpenWeather) | 4 (adds Tomorrow.io, Visual Crossing) | Phase 0 documents these as Level 3; design shows full capacity; MVP renders 2 |
| Current weather | NP-01: "We don't deliver weather" | Shows observations with provenance | Framed as "what our observation source reports" with full provenance; not a consumer weather display |
| Alerts | Deferred to Level 3 | Nav item present | Nav slot reserved; screen shows "Alerts available in a future release" empty state in MVP |
| Forecast changes | Not explicitly specified | Change feed with impact levels | **UI Discovered Requirement**: requires backend diff computation between successive forecast snapshots |
| Confidence rating | CI on metrics | "Confidence: Moderate" derived from provider agreement | Composite of CI width + provider disagreement; methodology link provided |

### UI Discovered Requirements (New)

| ID | Requirement | Priority |
|----|-------------|----------|
| DR-06 | Forecast change detection: backend must compute material deltas between successive snapshots for the same target_time | High |
| DR-07 | Forecast consensus: aggregation of provider forecasts into a single "consensus" value (mean/median) with agreement count | High |
| DR-08 | Confidence rating: derived metric combining CI width and provider disagreement into a human-readable rating (High/Moderate/Low) | Medium |
| DR-09 | Real-time observation summary: endpoint returning latest observation per location with derived feels-like temperature | Medium |
| DR-10 | Provider disagreement index: standard deviation across provider forecasts for a target period, exposed as a time series | Medium |
