"use client";

import { useCallback } from "react";
import { useApi } from "@/lib/api/hooks";
import { HealthGrid, type HealthCell } from "@/components/HealthGrid";
import { SkeletonBlock } from "@/components/SkeletonBlock";
import { ErrorPanel } from "@/components/ErrorPanel";

interface HealthData {
  cells: HealthCell[];
}

// S-10 Admin Health (doc 02 §4.10). Auto-refreshes every 60s (no-store;
// SWR refreshInterval). The admin layout handles the role guard.
export default function AdminHealthPage() {
  const { data: envelope, error, isLoading, mutate } = useApi<HealthData>("/admin/health", {
    refreshInterval: 60000, // 60s auto-refresh per doc 02 §14.3
  });

  const handleRetry = useCallback(async (providerId: string, locationId: string) => {
    try {
      await fetch("/api/v1/admin/collections/trigger", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ provider_id: providerId, location_id: locationId }),
      });
      mutate(); // Refresh after trigger.
    } catch { /* best-effort */ }
  }, [mutate]);

  if (isLoading) {
    return (
      <section aria-labelledby="health-heading">
        <h1 id="health-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Collection Health</h1>
        <SkeletonBlock variant="row" count={5} />
      </section>
    );
  }

  if (error) {
    return (
      <section aria-labelledby="health-heading">
        <h1 id="health-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Collection Health</h1>
        <ErrorPanel message="Unable to load health data." requestId={error.requestId} onRetry={() => mutate()} />
      </section>
    );
  }

  const cells = envelope?.data?.cells ?? [];

  return (
    <section aria-labelledby="health-heading">
      <h1 id="health-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Collection Health</h1>
      <p style={{ color: "var(--color-text-secondary)", fontSize: "var(--text-body-sm)", marginBottom: "var(--space-md)" }}>
        Auto-refreshes every 60 seconds. Retry triggers a manual collection for the selected cell.
      </p>
      {cells.length === 0 ? (
        <p style={{ color: "var(--color-text-secondary)" }}>No collection cells configured.</p>
      ) : (
        <HealthGrid cells={cells} onRetry={handleRetry} />
      )}
    </section>
  );
}
