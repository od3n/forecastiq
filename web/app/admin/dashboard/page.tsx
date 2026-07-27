"use client";

import { useApi } from "@/lib/api/hooks";
import { ConditionsTimeline } from "@/components/ConditionsTimeline";
import { SkeletonBlock } from "@/components/SkeletonBlock";
import { ErrorPanel } from "@/components/ErrorPanel";

interface ProviderRef { id: string; name: string; status: string; }
interface LocationRef { id: string; name: string; country_code: string; timezone?: string; status: string; }

// Admin Dashboard: per location × provider "Now ±12h" conditions strips —
// observed past hours (ground truth) vs. each provider's latest forecast
// ahead. Operator context only (NP-01). An AI-advise action per location will
// land here later.
export default function AdminDashboardPage() {
  const { data: locationsEnv, error: locError, isLoading: locLoading, mutate } = useApi<{ locations: LocationRef[] }>("/locations");
  const { data: providersEnv } = useApi<{ providers: ProviderRef[] }>("/providers");

  const locations = (locationsEnv?.data?.locations ?? []).filter((l) => l.status === "active");
  const providers = (providersEnv?.data?.providers ?? []).filter((p) => p.status === "active");

  if (locLoading) {
    return (
      <section>
        <h1 style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Dashboard</h1>
        <SkeletonBlock variant="row" count={6} />
      </section>
    );
  }

  if (locError) {
    return (
      <section>
        <h1 style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Dashboard</h1>
        <ErrorPanel message="Unable to load locations." requestId={locError.requestId} onRetry={() => mutate()} />
      </section>
    );
  }

  return (
    <section aria-labelledby="dashboard-heading">
      <h1 id="dashboard-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-sm)" }}>Dashboard</h1>
      <p style={{ color: "var(--color-text-secondary)", fontSize: "var(--text-body-sm)", marginBottom: "var(--space-lg)" }}>
        Per location: the last 12 observed hours (grey, ground truth) against each provider&rsquo;s latest forecast for the next 12 (white). Each hour shows condition, humidity, and temperature.
      </p>

      {locations.length === 0 && <p style={{ color: "var(--color-text-secondary)" }}>No active locations.</p>}

      {locations.map((loc) => (
        <section key={loc.id} aria-labelledby={`loc-${loc.id}`} style={{ marginBottom: "var(--space-xl)", padding: "var(--space-md)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)" }}>
          <div style={{ display: "flex", alignItems: "baseline", gap: "var(--space-sm)", marginBottom: "var(--space-md)" }}>
            <h2 id={`loc-${loc.id}`} style={{ fontSize: "var(--text-h1)", fontWeight: 600, margin: 0 }}>
              {loc.name}
            </h2>
            <span style={{ fontFamily: "var(--font-data)", fontSize: "var(--text-body-sm)", color: "var(--color-text-secondary)" }}>
              {loc.country_code}{loc.timezone ? ` · ${loc.timezone}` : ""}
            </span>
            {/* AI-advise action lands here (future). */}
          </div>

          {providers.map((p) => (
            <div key={p.id} style={{ marginBottom: "var(--space-md)" }}>
              <h3 style={{ fontSize: "var(--text-body-sm)", fontWeight: 600, marginBottom: "var(--space-xs)", color: "var(--color-text-secondary)" }}>
                {p.name}
              </h3>
              <ConditionsTimeline providerId={p.id} locationId={loc.id} timezone={loc.timezone} hideHeading />
            </div>
          ))}
        </section>
      ))}
    </section>
  );
}
