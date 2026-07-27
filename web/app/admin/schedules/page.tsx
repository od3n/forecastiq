"use client";

import { useApi } from "@/lib/api/hooks";
import { SkeletonBlock } from "@/components/SkeletonBlock";
import { ErrorPanel } from "@/components/ErrorPanel";

interface HealthCell {
  provider: { id: string; name: string };
  location_id: string;
  location_name: string;
  last_success_at: string | null;
  last_status: string;
  freshness: { state: string; last_updated?: string; age_seconds?: number };
}

interface HealthData {
  cells: HealthCell[];
}

// Admin Schedules page — shows collection schedule status per provider × location
// derived from the health endpoint (no dedicated schedules API exists yet).
export default function AdminSchedulesPage() {
  const { data: envelope, error, isLoading, mutate } = useApi<HealthData>("/admin/health");

  if (isLoading) return (<section><h1 style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Schedules</h1><SkeletonBlock variant="row" count={5} /></section>);
  if (error) return (<section><h1 style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Schedules</h1><ErrorPanel message="Unable to load schedules." requestId={error.requestId} onRetry={() => mutate()} /></section>);

  const cells = envelope?.data?.cells ?? [];

  return (
    <section>
      <h1 style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Schedules</h1>
      <p style={{ color: "var(--color-text-secondary)", fontSize: "var(--text-body-sm)", marginBottom: "var(--space-md)" }}>
        Collection runs hourly per provider-location cell. Schedule configuration is managed via provider configurations.
      </p>
      {cells.length === 0 ? (
        <p style={{ color: "var(--color-text-secondary)" }}>No collection schedules active.</p>
      ) : (
        <div className="tableWrap">
          <table style={{ width: "100%", borderCollapse: "collapse", minWidth: 500 }}>
            <thead>
              <tr style={{ background: "var(--color-surface-secondary)", fontSize: "var(--text-label)", textTransform: "uppercase" }}>
                <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Provider</th>
                <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Location</th>
                <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Interval</th>
                <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Last Collection</th>
                <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Status</th>
              </tr>
            </thead>
            <tbody>
              {cells.map((c) => (
                <tr key={`${c.provider.id}-${c.location_id}`} style={{ borderBottom: "1px solid var(--color-border)", height: 44 }}>
                  <td style={{ padding: "var(--space-sm)", fontWeight: 500 }}>{c.provider.name}</td>
                  <td style={{ padding: "var(--space-sm)" }}>{c.location_name}</td>
                  <td style={{ padding: "var(--space-sm)", fontFamily: "var(--font-data)" }}>Hourly</td>
                  <td style={{ padding: "var(--space-sm)", fontFamily: "var(--font-data)", fontSize: "var(--text-body-sm)" }}>
                    {c.last_success_at ? new Date(c.last_success_at).toLocaleString() : "Never"}
                  </td>
                  <td style={{ padding: "var(--space-sm)" }}>
                    <span style={{
                      padding: "2px var(--space-sm)",
                      borderRadius: "var(--radius-full)",
                      fontSize: "var(--text-label)",
                      fontWeight: 500,
                      background: c.last_status === "partial" || c.last_status === "success" ? "var(--color-ranked)" : c.last_status === "failed" ? "var(--color-unavailable)" : "var(--color-border)",
                      color: "#fff",
                    }}>
                      {c.last_status || "pending"}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
