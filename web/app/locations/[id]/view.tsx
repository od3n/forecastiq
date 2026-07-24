"use client";

import { Suspense } from "react";
import { useGlobalParams } from "@/lib/state/use-global-params";
import { useApi } from "@/lib/api/hooks";
import { MetricTable, type MetricRow } from "@/components/MetricTable";
import { SkeletonBlock } from "@/components/SkeletonBlock";
import { EmptyState } from "@/components/EmptyState";
import { ErrorPanel } from "@/components/ErrorPanel";
import { StaleBanner } from "@/components/StaleBanner";
import { PartialWarnings } from "@/components/PartialWarnings";
import { AttributionFooter } from "@/components/AttributionFooter";
import type { Freshness, Warning, Attribution } from "@/lib/api/types";

interface VariableMetrics {
  variable: string;
  unit: string;
  providers: MetricRow[];
}

interface AccuracySummaryData {
  metrics: VariableMetrics[];
  location_name?: string;
}

const VARIABLES = ["temperature", "precipitation", "wind_speed", "humidity", "pressure"];
const UNITS: Record<string, string> = {
  temperature: "°C",
  precipitation: "mm",
  wind_speed: "m/s",
  humidity: "%",
  pressure: "hPa",
};

function LocationDetailContent({ locationId }: { locationId: string }) {
  const { horizonMinutes } = useGlobalParams();
  const path = `/accuracy/summary?location_id=${locationId}&horizon_minutes=${horizonMinutes}`;
  const { data: envelope, error, isLoading } = useApi<AccuracySummaryData>(path);

  if (isLoading) {
    return (
      <section aria-labelledby="loc-heading" aria-busy="true">
        <h1 id="loc-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>
          Location Detail
        </h1>
        <SkeletonBlock variant="card" count={2} />
        <div style={{ marginTop: "var(--space-md)" }}>
          <SkeletonBlock variant="row" count={5} />
        </div>
      </section>
    );
  }

  if (error) {
    return (
      <section aria-labelledby="loc-heading">
        <h1 id="loc-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>
          Location Detail
        </h1>
        <ErrorPanel
          message="Unable to load location data."
          requestId={error.requestId}
          onRetry={() => window.location.reload()}
        />
      </section>
    );
  }

  const metrics = envelope?.data?.metrics ?? [];
  const locationName = envelope?.data?.location_name ?? "Location";
  const freshness = envelope?.freshness as Freshness | undefined;
  const warnings = envelope?.warnings as Warning[] | undefined;
  const attribution = envelope?.attribution as Attribution[] | undefined;
  const methodology = envelope?.metadata?.methodology_version;

  if (metrics.length === 0) {
    return (
      <section aria-labelledby="loc-heading">
        <h1 id="loc-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>
          {locationName}
        </h1>
        <EmptyState
          variant="no-data"
          title="Collecting data"
          description="First accuracy data appears after ~7 days of matched forecasts and observations."
        />
      </section>
    );
  }

  const grouped = metrics.length > 0 && metrics[0].variable
    ? metrics
    : VARIABLES.map((v) => ({ variable: v, unit: UNITS[v] ?? "", providers: [] as MetricRow[] }));

  return (
    <section aria-labelledby="loc-heading">
      <h1 id="loc-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>
        {locationName}
      </h1>

      {freshness && freshness.state === "stale" && freshness.last_updated && (
        <div style={{ marginBottom: "var(--space-md)" }}>
          <StaleBanner lastUpdated={freshness.last_updated} />
        </div>
      )}

      {warnings && warnings.length > 0 && (
        <div style={{ marginBottom: "var(--space-md)" }}>
          <PartialWarnings warnings={warnings} />
        </div>
      )}

      {grouped.filter((g) => g.providers.length > 0).map((g) => (
        <MetricTable key={g.variable} variable={g.variable} unit={g.unit} rows={g.providers} />
      ))}

      {attribution && (
        <div style={{ marginTop: "var(--space-xl)" }}>
          <AttributionFooter providers={attribution} methodologyVersion={methodology} />
        </div>
      )}
    </section>
  );
}

// Client wrapper for the Location Detail screen (S-02). The page.tsx server
// component passes locationId from the dynamic [id] param.
export function LocationDetailView({ locationId }: { locationId: string }) {
  return (
    <Suspense fallback={<SkeletonBlock variant="row" count={5} />}>
      <LocationDetailContent locationId={locationId} />
    </Suspense>
  );
}
