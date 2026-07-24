"use client";

import { Suspense } from "react";
import { useGlobalParams } from "@/lib/state/use-global-params";
import { useApi } from "@/lib/api/hooks";
import { RankingTable, type RankingEntry } from "@/components/RankingTable";
import { SkeletonBlock } from "@/components/SkeletonBlock";
import { EmptyState } from "@/components/EmptyState";
import { ErrorPanel } from "@/components/ErrorPanel";
import { StaleBanner } from "@/components/StaleBanner";
import { PartialWarnings } from "@/components/PartialWarnings";
import { AttributionFooter } from "@/components/AttributionFooter";
import type { Freshness, Warning, Attribution } from "@/lib/api/types";

interface RankingsData {
  rankings: RankingEntry[];
}

// S-01 Overview (Rankings). Displays the live ranking cohort for the selected
// location + horizon. All 9 mandatory states from the state contracts are
// handled: loading, no-locations, no-data, insufficient (per-row), provisional
// (per-row), partial (banner + badge), stale (banner), error (panel + retry),
// observation-unavailable (context note).
function OverviewContent() {
  const { locationId, horizonMinutes } = useGlobalParams();

  const path = locationId
    ? `/rankings?location_id=${locationId}&horizon_minutes=${horizonMinutes}`
    : null;

  const { data: envelope, error, isLoading } = useApi<RankingsData>(path);

  // Loading state.
  if (isLoading) {
    return (
      <section aria-labelledby="overview-heading" aria-busy="true">
        <h1 id="overview-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>
          Overview
        </h1>
        <SkeletonBlock variant="card" count={3} />
        <div style={{ marginTop: "var(--space-md)" }}>
          <SkeletonBlock variant="row" count={4} />
        </div>
      </section>
    );
  }

  // Error state.
  if (error) {
    return (
      <section aria-labelledby="overview-heading">
        <h1 id="overview-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>
          Overview
        </h1>
        <ErrorPanel
          message="The server may be temporarily unavailable."
          requestId={error.requestId}
          onRetry={() => window.location.reload()}
        />
      </section>
    );
  }

  const rankings = envelope?.data?.rankings ?? [];
  const freshness = envelope?.freshness as Freshness | undefined;
  const warnings = envelope?.warnings as Warning[] | undefined;
  const attribution = envelope?.attribution as Attribution[] | undefined;
  const methodology = envelope?.metadata?.methodology_version;

  // No locations selected (locationId null and no auto-select yet).
  if (!locationId) {
    return (
      <section aria-labelledby="overview-heading">
        <h1 id="overview-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>
          Overview
        </h1>
        <EmptyState variant="no-locations" title="No locations monitored yet" description="Locations are added by the platform operator." />
      </section>
    );
  }

  // No data yet (empty rankings).
  if (rankings.length === 0) {
    return (
      <section aria-labelledby="overview-heading">
        <h1 id="overview-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>
          Overview
        </h1>
        <EmptyState
          variant="no-data"
          title="Collecting data"
          description="First data appears within ~1 hour. Rankings require 7+ days of matched data."
        />
      </section>
    );
  }

  return (
    <section aria-labelledby="overview-heading">
      <h1 id="overview-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>
        Overview
      </h1>

      {/* Stale banner (persistent, non-dismissible). */}
      {freshness && (freshness.state === "stale") && freshness.last_updated && (
        <div style={{ marginBottom: "var(--space-md)" }}>
          <StaleBanner lastUpdated={freshness.last_updated} />
        </div>
      )}

      {/* Partial-result warnings. */}
      {warnings && warnings.length > 0 && (
        <div style={{ marginBottom: "var(--space-md)" }}>
          <PartialWarnings warnings={warnings} />
        </div>
      )}

      {/* Ranking table. */}
      <RankingTable rankings={rankings} freshness={freshness} methodologyVersion={methodology} />

      {/* Attribution footer wired with real envelope data. */}
      {attribution && (
        <div style={{ marginTop: "var(--space-xl)" }}>
          <AttributionFooter
            providers={attribution}
            methodologyVersion={methodology}
          />
        </div>
      )}
    </section>
  );
}

export default function OverviewPage() {
  return (
    <Suspense fallback={<SkeletonBlock variant="card" count={3} />}>
      <OverviewContent />
    </Suspense>
  );
}
