"use client";

import { FreshnessBadge } from "./FreshnessBadge";
import { absoluteLocal } from "@/lib/format";
import type { FreshnessState } from "@/lib/api/types";

export interface HealthCell {
  provider_name: string;
  location_name: string;
  last_success: string | null;
  status: string;
  freshness: FreshnessState;
  circuit_state: string;
  next_scheduled_at: string | null;
  provider_id: string;
  location_id: string;
}

export interface HealthGridProps {
  cells: HealthCell[];
  onRetry: (providerId: string, locationId: string) => void;
}

// S-10 Admin Health grid (doc 02 §4.10). Per provider-location cell: freshness,
// last success, circuit state, next scheduled. Retry button per cell
// (aria-label specific per doc 07 §2 S-10). Auto-refresh is handled by the page
// (SWR refreshInterval). State changes announced via aria-live (A-06).
export function HealthGrid({ cells, onRetry }: HealthGridProps) {
  return (
    <div className="tableWrap" aria-live="polite" aria-atomic="false">
      <table style={{ width: "100%", borderCollapse: "collapse", minWidth: 700 }}>
        <thead>
          <tr style={{ background: "var(--color-surface-secondary)", fontSize: "var(--text-label)", textTransform: "uppercase" }}>
            <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Provider</th>
            <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Location</th>
            <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Freshness</th>
            <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Last Success</th>
            <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Circuit</th>
            <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Next Scheduled</th>
            <th scope="col" style={{ padding: "var(--space-sm)" }}><span className="sr-only">Actions</span></th>
          </tr>
        </thead>
        <tbody>
          {cells.map((c) => (
            <tr key={`${c.provider_id}-${c.location_id}`} style={{ borderBottom: "1px solid var(--color-border)", height: 44 }}>
              <td style={{ padding: "var(--space-sm)" }}>{c.provider_name}</td>
              <td style={{ padding: "var(--space-sm)" }}>{c.location_name}</td>
              <td style={{ padding: "var(--space-sm)" }}>
                <FreshnessBadge state={c.freshness} lastUpdated={c.last_success ?? undefined} />
              </td>
              <td style={{ padding: "var(--space-sm)", fontFamily: "var(--font-data)", fontSize: "var(--text-body-sm)" }}>
                {c.last_success ? absoluteLocal(c.last_success) : "—"}
              </td>
              <td style={{ padding: "var(--space-sm)", fontWeight: c.circuit_state === "open" ? 700 : 400, color: c.circuit_state === "open" ? "var(--color-unavailable)" : undefined }}>
                {c.circuit_state}
              </td>
              <td style={{ padding: "var(--space-sm)", fontFamily: "var(--font-data)", fontSize: "var(--text-body-sm)" }}>
                {c.next_scheduled_at ? absoluteLocal(c.next_scheduled_at) : "—"}
              </td>
              <td style={{ padding: "var(--space-sm)" }}>
                <button
                  type="button"
                  onClick={() => onRetry(c.provider_id, c.location_id)}
                  aria-label={`Re-collect now for ${c.provider_name} at ${c.location_name}`}
                  style={{ padding: "var(--space-xs) var(--space-sm)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-sm)", background: "var(--color-surface)", fontFamily: "inherit", fontSize: "var(--text-label)", cursor: "pointer" }}
                >
                  Retry
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
