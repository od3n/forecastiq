"use client";

import { Suspense, useMemo } from "react";
import { useApi } from "@/lib/api/hooks";
import { ProviderGrid, type GridCell } from "@/components/ProviderGrid";
import { SkeletonBlock } from "@/components/SkeletonBlock";
import { EmptyState } from "@/components/EmptyState";
import { ErrorPanel } from "@/components/ErrorPanel";
import { StaleBanner } from "@/components/StaleBanner";
import { PartialWarnings } from "@/components/PartialWarnings";
import { AttributionFooter } from "@/components/AttributionFooter";
import type { Freshness, Warning, Attribution } from "@/lib/api/types";

interface ApiSummaryCell {
  location_id: string;
  location_name: string;
  horizon_minutes: number;
  composite_score: number | null;
  ranking_status: string;
  sample_count: number;
}

interface ProviderSummaryData {
  provider: { id: string; name: string; slug: string };
  cells: ApiSummaryCell[];
}

function ProviderDetailContent({ providerId }: { providerId: string }) {
  const path = `/accuracy/summary?provider_id=${providerId}`;
  const { data: envelope, error, isLoading } = useApi<ProviderSummaryData>(path);

  // Group flat location×horizon rows into per-location score maps for the grid.
  const cells: GridCell[] = useMemo(() => {
    const byLocation = new Map<string, GridCell>();
    for (const c of envelope?.data?.cells ?? []) {
      if (!byLocation.has(c.location_id)) {
        byLocation.set(c.location_id, { location_id: c.location_id, location_name: c.location_name, scores: {} });
      }
      byLocation.get(c.location_id)!.scores[c.horizon_minutes] = c.composite_score;
    }
    return Array.from(byLocation.values());
  }, [envelope]);

  if (isLoading) {
    return (
      <section aria-labelledby="prov-heading" aria-busy="true">
        <h1 id="prov-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Provider Detail</h1>
        <SkeletonBlock variant="card" count={2} />
        <div style={{ marginTop: "var(--space-md)" }}><SkeletonBlock variant="row" count={5} /></div>
      </section>
    );
  }

  if (error) {
    return (
      <section aria-labelledby="prov-heading">
        <h1 id="prov-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Provider Detail</h1>
        <ErrorPanel message="Unable to load provider data." requestId={error.requestId} onRetry={() => window.location.reload()} />
      </section>
    );
  }

  const providerName = envelope?.data?.provider?.name ?? "Provider";
  const freshness = envelope?.freshness as Freshness | undefined;
  const warnings = envelope?.warnings as Warning[] | undefined;
  const attribution = envelope?.attribution as Attribution[] | undefined;
  const methodology = envelope?.metadata?.methodology_version;

  if (cells.length === 0) {
    return (
      <section aria-labelledby="prov-heading">
        <h1 id="prov-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>{providerName}</h1>
        <EmptyState variant="no-data" title="No data yet" description="Performance data appears after 7+ days of collections across locations." />
      </section>
    );
  }

  return (
    <section aria-labelledby="prov-heading">
      <h1 id="prov-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>{providerName}</h1>
      {freshness?.state === "stale" && freshness.last_updated && (
        <div style={{ marginBottom: "var(--space-md)" }}><StaleBanner lastUpdated={freshness.last_updated} /></div>
      )}
      {warnings && warnings.length > 0 && (
        <div style={{ marginBottom: "var(--space-md)" }}><PartialWarnings warnings={warnings} /></div>
      )}
      <ProviderGrid providerName={providerName} cells={cells} />
      {attribution && (
        <div style={{ marginTop: "var(--space-xl)" }}><AttributionFooter providers={attribution} methodologyVersion={methodology} /></div>
      )}
    </section>
  );
}

export function ProviderDetailView({ providerId }: { providerId: string }) {
  return (
    <Suspense fallback={<SkeletonBlock variant="row" count={5} />}>
      <ProviderDetailContent providerId={providerId} />
    </Suspense>
  );
}
