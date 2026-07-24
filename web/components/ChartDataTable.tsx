"use client";

import type { ReactNode } from "react";

export interface ChartDataPoint {
  period_start: string;
  [providerKey: string]: string | number | null | undefined;
}

export interface ChartDataTableProps {
  data: ChartDataPoint[];
  providers: string[];
  unit: string;
}

// Visually-hidden accessible data table that provides screen readers with the
// full chart values (doc 07 §2 S-04). Rendered alongside the chart, which
// carries role="img" + aria-label for the visual summary.
export function ChartDataTable({ data, providers, unit }: ChartDataTableProps) {
  return (
    <table
      className="sr-only"
      aria-label={`Trend data table (${unit})`}
      style={{ position: "absolute", width: 1, height: 1, overflow: "hidden", clip: "rect(0,0,0,0)", whiteSpace: "nowrap" }}
    >
      <thead>
        <tr>
          <th scope="col">Period</th>
          {providers.map((p) => (
            <th key={p} scope="col">{p}</th>
          ))}
        </tr>
      </thead>
      <tbody>
        {data.map((row) => (
          <tr key={row.period_start}>
            <td>{row.period_start}</td>
            {providers.map((p) => (
              <td key={p}>{row[p] != null ? String(row[p]) : "—"}</td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}

// Wrapper providing the sr-only style via a global className (defined in globals.css).
export function ChartWrapper({ children, label }: { children: ReactNode; label: string }) {
  return (
    <div style={{ position: "relative" }} role="img" aria-label={label}>
      {children}
    </div>
  );
}
