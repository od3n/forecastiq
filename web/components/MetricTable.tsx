"use client";

export interface MetricRow {
  provider_id: string;
  provider_name: string;
  mae: number | null;
  rmse: number | null;
  bias: number | null;
  sample_count: number;
}

export interface MetricTableProps {
  variable: string;
  unit: string;
  rows: MetricRow[];
}

// Per-variable metric table (S-02 Location Detail; doc 02 §4.2). Providers as
// rows; metrics as columns (MAE, RMSE, Bias, samples). Null values rendered as
// "—" with a focusable tooltip explaining the absence (a11y §2 S-02).
export function MetricTable({ variable, unit, rows }: MetricTableProps) {
  return (
    <section aria-labelledby={`metric-${variable}`} style={{ marginBottom: "var(--space-lg)" }}>
      <h3
        id={`metric-${variable}`}
        style={{ fontSize: "var(--text-h2)", fontWeight: 600, marginBottom: "var(--space-sm)", textTransform: "capitalize" }}
      >
        {variable} ({unit})
      </h3>
      <table style={{ width: "100%", borderCollapse: "collapse" }}>
        <thead>
          <tr style={{ background: "var(--color-surface-secondary)", fontSize: "var(--text-label)", textTransform: "uppercase" }}>
            <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Provider</th>
            <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "right" }}>MAE</th>
            <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "right" }}>RMSE</th>
            <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "right" }}>Bias</th>
            <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "right" }}>Samples</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.provider_id} style={{ borderBottom: "1px solid var(--color-border)", height: 44 }}>
              <td style={{ padding: "var(--space-sm)" }}>{r.provider_name}</td>
              <MetricCell value={r.mae} />
              <MetricCell value={r.rmse} />
              <MetricCell value={r.bias} signed />
              <td style={{ padding: "var(--space-sm)", textAlign: "right", fontFamily: "var(--font-data)" }}>
                {r.sample_count}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}

function MetricCell({ value, signed }: { value: number | null; signed?: boolean }) {
  if (value === null) {
    return (
      <td
        style={{ padding: "var(--space-sm)", textAlign: "right", fontFamily: "var(--font-data)", color: "var(--color-text-muted)" }}
        tabIndex={0}
        title="No events in period — metric excluded per methodology"
        aria-label="No data"
      >
        —
      </td>
    );
  }
  const formatted = signed && value > 0 ? `+${value.toFixed(2)}` : value.toFixed(2);
  return (
    <td style={{ padding: "var(--space-sm)", textAlign: "right", fontFamily: "var(--font-data)" }}>
      {formatted}
    </td>
  );
}
