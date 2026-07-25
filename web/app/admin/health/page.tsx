"use client";

import { useCallback, useMemo } from "react";
import { useApi } from "@/lib/api/hooks";
import { HealthGrid, type HealthCell } from "@/components/HealthGrid";
import { SkeletonBlock } from "@/components/SkeletonBlock";
import { ErrorPanel } from "@/components/ErrorPanel";
import type { Freshness } from "@/lib/api/types";

import { apiBase } from "@/lib/api/client";

interface ApiCell {
  provider: { id: string; name: string; slug: string };
  location_id: string;
  location_name: string;
  last_success_at: string | null;
  last_status: string;
  next_scheduled_at?: string | null;
  freshness: Freshness;
}

interface ApiCircuit {
  provider_id: string;
  provider_name: string;
  state: string;
  consecutive_failures: number;
}

interface HealthData {
  cells: ApiCell[];
  circuits: ApiCircuit[];
}

// S-10 Admin Health (doc 02 §4.10). Auto-refreshes every 60s (no-store;
// SWR refreshInterval). The admin layout handles the role guard.
export default function AdminHealthPage() {
  const { data: envelope, error, isLoading, mutate } = useApi<HealthData>("/admin/health", {
    refreshInterval: 60000, // 60s auto-refresh per doc 02 §14.3
  });

  const handleRetry = useCallback(async (providerId: string, locationId: string) => {
    const token = process.env.NEXT_PUBLIC_DEV_TOKEN;
    const headers: Record<string, string> = { "Content-Type": "application/json" };
    if (token) headers["Authorization"] = `Bearer ${token}`;
    try {
      await fetch(`${apiBase}/admin/collections/trigger`, {
        method: "POST",
        headers,
        body: JSON.stringify({ provider_id: providerId, location_id: locationId }),
      });
      mutate(); // Refresh after trigger.
    } catch { /* best-effort */ }
  }, [mutate]);

  const cells: HealthCell[] = useMemo(() => {
    const apiCells = envelope?.data?.cells ?? [];
    const circuits = envelope?.data?.circuits ?? [];
    const circuitMap = new Map(circuits.map((c) => [c.provider_id, c.state]));

    return apiCells.map((c) => ({
      provider_name: c.provider.name,
      provider_id: c.provider.id,
      location_id: c.location_id,
      location_name: c.location_name,
      last_success: c.last_success_at,
      status: c.last_status,
      freshness: c.freshness?.state ?? "unavailable",
      circuit_state: circuitMap.get(c.provider.id) ?? "unknown",
      next_scheduled_at: c.next_scheduled_at ?? null,
    }));
  }, [envelope]);

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
