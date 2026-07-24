"use client";

import { HORIZON_OPTIONS } from "@/lib/state/use-global-params";

export interface GridCell {
  location_id: string;
  location_name: string;
  scores: Record<number, number | null>; // horizon_minutes -> score
}

export interface ProviderGridProps {
  providerName: string;
  cells: GridCell[];
}

// Cross-location/horizon grid for one provider (S-03; doc 02 §4.3). Locations
// as rows, horizons as columns. Cells show composite score; null → "—". Text
// value always present (not color-only). On mobile: horizontal scroll.
export function ProviderGrid({ providerName, cells }: ProviderGridProps) {
  const horizons = HORIZON_OPTIONS;

  return (
    <section aria-labelledby="grid-heading">
      <h2 id="grid-heading" style={{ fontSize: "var(--text-h1)", fontWeight: 600, marginBottom: "var(--space-md)" }}>
        {providerName} — Cross-Location Performance
      </h2>
      <div className="tableWrap">
        <table style={{ width: "100%", borderCollapse: "collapse", minWidth: 600 }}>
          <thead>
            <tr style={{ background: "var(--color-surface-secondary)", fontSize: "var(--text-label)", textTransform: "uppercase" }}>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Location</th>
              {horizons.map((h) => (
                <th key={h.minutes} scope="col" style={{ padding: "var(--space-sm)", textAlign: "right" }}>
                  {h.label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {cells.map((cell) => (
              <tr key={cell.location_id} style={{ borderBottom: "1px solid var(--color-border)", height: 44 }}>
                <td style={{ padding: "var(--space-sm)" }}>{cell.location_name}</td>
                {horizons.map((h) => {
                  const score = cell.scores[h.minutes];
                  return (
                    <td
                      key={h.minutes}
                      style={{ padding: "var(--space-sm)", textAlign: "right", fontFamily: "var(--font-data)" }}
                    >
                      {score !== null && score !== undefined ? score.toFixed(3) : "—"}
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
