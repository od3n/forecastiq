"use client";

export interface DayMetric {
  provider_name: string;
  mae: number | null;
  rmse: number | null;
  bias: number | null;
}

export interface DayMetricsTableProps {
  metrics: DayMetric[];
  unit: string;
}

// Day-level metrics summary table below the FvA chart (S-05; doc 02 §5.3).
// Per-provider MAE, RMSE, Bias for the selected day. Mono font for values.
export function DayMetricsTable({ metrics, unit }: DayMetricsTableProps) {
  if (metrics.length === 0) return null;
  return (
    <section aria-labelledby="day-metrics-heading" style={{ marginTop: "var(--space-lg)" }}>
      <h3 id="day-metrics-heading" style={{ fontSize: "var(--text-h2)", fontWeight: 600, marginBottom: "var(--space-sm)" }}>
        Day Metrics ({unit})
      </h3>
      <table style={{ width: "100%", borderCollapse: "collapse" }}>
        <thead>
          <tr style={{ background: "var(--color-surface-secondary)", fontSize: "var(--text-label)", textTransform: "uppercase" }}>
            <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Provider</th>
            <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "right" }}>MAE</th>
            <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "right" }}>RMSE</th>
            <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "right" }}>Bias</th>
          </tr>
        </thead>
        <tbody>
          {metrics.map((m) => (
            <tr key={m.provider_name} style={{ borderBottom: "1px solid var(--color-border)", height: 44 }}>
              <td style={{ padding: "var(--space-sm)" }}>{m.provider_name}</td>
              <td style={{ padding: "var(--space-sm)", textAlign: "right", fontFamily: "var(--font-data)" }}>
                {m.mae !== null ? m.mae.toFixed(2) : "—"}
              </td>
              <td style={{ padding: "var(--space-sm)", textAlign: "right", fontFamily: "var(--font-data)" }}>
                {m.rmse !== null ? m.rmse.toFixed(2) : "—"}
              </td>
              <td style={{ padding: "var(--space-sm)", textAlign: "right", fontFamily: "var(--font-data)" }}>
                {m.bias !== null ? (m.bias > 0 ? `+${m.bias.toFixed(2)}` : m.bias.toFixed(2)) : "—"}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}
