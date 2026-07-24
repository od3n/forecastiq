// Overview (S-01) — foundation placeholder. The full ranking view, its states,
// and data wiring land in WP-21; the app shell, tokens, and primitives are the
// WP-20 deliverable.
export default function OverviewPage() {
  return (
    <section aria-labelledby="overview-heading">
      <h1 id="overview-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700 }}>
        Overview
      </h1>
      <p style={{ color: "var(--color-text-secondary)", marginTop: "var(--space-sm)" }}>
        Forecast-accuracy rankings for the selected location and horizon appear here.
        The dashboard foundation (app shell, design tokens, API client, auth, and
        rendering primitives) is in place; the screens land in the next work package.
      </p>
    </section>
  );
}
