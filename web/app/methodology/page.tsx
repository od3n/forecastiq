"use client";

import { Suspense } from "react";
import { useApi } from "@/lib/api/hooks";
import { SkeletonBlock } from "@/components/SkeletonBlock";
import { ErrorPanel } from "@/components/ErrorPanel";

interface Formula {
  metric_type: string;
  formula: string;
  plain_language: string;
  direction: string;
  zero_denominator_behaviour: string;
  anchor: string;
  ranked: boolean;
}

interface Weight {
  component: string;
  weight: number;
  direction: string;
}

interface MethodologyData {
  methodology_version: string;
  weights_version: string;
  formulas: Formula[];
  default_weights: Weight[];
  thresholds: {
    provisional: number;
    ranked: number;
  };
  coverage_penalty: {
    no_penalty_at_or_above: number;
    penalty_range: string;
    formula: string;
    unranked_below: number;
    outranking_rule: string;
  };
  statuses: { status: string; condition: string }[];
  tie_rule: string;
  rounding: Record<string, string>;
  docs: string;
}

function MethodologyContent() {
  const { data: envelope, error, isLoading } = useApi<MethodologyData>("/rankings/methodology");

  if (isLoading) {
    return (
      <article aria-labelledby="meth-heading" aria-busy="true">
        <h1 id="meth-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Methodology</h1>
        <SkeletonBlock variant="row" count={8} />
      </article>
    );
  }

  if (error) {
    return (
      <article aria-labelledby="meth-heading">
        <h1 id="meth-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Methodology</h1>
        <ErrorPanel message="Unable to load methodology." requestId={error.requestId} onRetry={() => window.location.reload()} />
      </article>
    );
  }

  const m = envelope?.data;
  if (!m) return null;

  return (
    <article aria-labelledby="meth-heading">
      <h1 id="meth-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Methodology</h1>
      <p style={{ color: "var(--color-text-secondary)", marginBottom: "var(--space-lg)" }}>
        Version {m.methodology_version} · Weights {m.weights_version}
      </p>

      {/* Formulas */}
      <section aria-labelledby="formula-heading" style={{ marginBottom: "var(--space-xl)" }}>
        <h2 id="formula-heading" style={{ fontSize: "var(--text-h1)", fontWeight: 600, marginBottom: "var(--space-sm)" }}>Metric Formulas</h2>
        <table style={{ width: "100%", borderCollapse: "collapse" }}>
          <thead>
            <tr style={{ background: "var(--color-surface-secondary)", fontSize: "var(--text-label)", textTransform: "uppercase" }}>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Metric</th>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Formula</th>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Description</th>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "center" }}>Ranked</th>
            </tr>
          </thead>
          <tbody>
            {m.formulas.map((f) => (
              <tr key={f.metric_type} style={{ borderBottom: "1px solid var(--color-border)" }}>
                <td style={{ padding: "var(--space-sm)", fontFamily: "var(--font-data)", fontWeight: 500 }}>{f.metric_type}</td>
                <td style={{ padding: "var(--space-sm)", fontFamily: "var(--font-data)" }}>{f.formula}</td>
                <td style={{ padding: "var(--space-sm)", color: "var(--color-text-secondary)" }}>{f.plain_language}</td>
                <td style={{ padding: "var(--space-sm)", textAlign: "center" }}>{f.ranked ? "Yes" : "—"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>

      {/* Weights */}
      <section aria-labelledby="weights-heading" style={{ marginBottom: "var(--space-xl)" }}>
        <h2 id="weights-heading" style={{ fontSize: "var(--text-h1)", fontWeight: 600, marginBottom: "var(--space-sm)" }}>Default Weights</h2>
        <table style={{ width: "100%", borderCollapse: "collapse" }}>
          <thead>
            <tr style={{ background: "var(--color-surface-secondary)", fontSize: "var(--text-label)", textTransform: "uppercase" }}>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Component</th>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "right" }}>Weight</th>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Direction</th>
            </tr>
          </thead>
          <tbody>
            {m.default_weights.map((w) => (
              <tr key={w.component} style={{ borderBottom: "1px solid var(--color-border)" }}>
                <td style={{ padding: "var(--space-sm)" }}>{w.component}</td>
                <td style={{ padding: "var(--space-sm)", textAlign: "right", fontFamily: "var(--font-data)" }}>{w.weight}</td>
                <td style={{ padding: "var(--space-sm)", color: "var(--color-text-secondary)" }}>{w.direction}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>

      {/* Thresholds */}
      <section aria-labelledby="thresholds-heading" style={{ marginBottom: "var(--space-xl)" }}>
        <h2 id="thresholds-heading" style={{ fontSize: "var(--text-h1)", fontWeight: 600, marginBottom: "var(--space-sm)" }}>Thresholds</h2>
        <dl style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "var(--space-sm)" }}>
          <dt style={{ color: "var(--color-text-secondary)" }}>Provisional (min pairs)</dt>
          <dd style={{ fontFamily: "var(--font-data)", margin: 0 }}>{m.thresholds.provisional}</dd>
          <dt style={{ color: "var(--color-text-secondary)" }}>Ranked (min pairs)</dt>
          <dd style={{ fontFamily: "var(--font-data)", margin: 0 }}>{m.thresholds.ranked}</dd>
        </dl>
      </section>

      {/* Coverage penalty */}
      <section aria-labelledby="coverage-heading" style={{ marginBottom: "var(--space-xl)" }}>
        <h2 id="coverage-heading" style={{ fontSize: "var(--text-h1)", fontWeight: 600, marginBottom: "var(--space-sm)" }}>Coverage Penalty</h2>
        <dl style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "var(--space-sm)" }}>
          <dt style={{ color: "var(--color-text-secondary)" }}>No penalty at or above</dt>
          <dd style={{ fontFamily: "var(--font-data)", margin: 0 }}>{(m.coverage_penalty.no_penalty_at_or_above * 100).toFixed(0)}%</dd>
          <dt style={{ color: "var(--color-text-secondary)" }}>Penalty range</dt>
          <dd style={{ fontFamily: "var(--font-data)", margin: 0 }}>{m.coverage_penalty.penalty_range}</dd>
          <dt style={{ color: "var(--color-text-secondary)" }}>Formula</dt>
          <dd style={{ fontFamily: "var(--font-data)", margin: 0 }}>{m.coverage_penalty.formula}</dd>
          <dt style={{ color: "var(--color-text-secondary)" }}>Unranked below</dt>
          <dd style={{ fontFamily: "var(--font-data)", margin: 0 }}>{(m.coverage_penalty.unranked_below * 100).toFixed(0)}%</dd>
        </dl>
        <p style={{ color: "var(--color-text-secondary)", marginTop: "var(--space-sm)", fontSize: "var(--text-body-sm)" }}>
          {m.coverage_penalty.outranking_rule}
        </p>
      </section>

      {/* Ranking statuses */}
      <section aria-labelledby="statuses-heading" style={{ marginBottom: "var(--space-xl)" }}>
        <h2 id="statuses-heading" style={{ fontSize: "var(--text-h1)", fontWeight: 600, marginBottom: "var(--space-sm)" }}>Ranking Statuses</h2>
        <table style={{ width: "100%", borderCollapse: "collapse" }}>
          <thead>
            <tr style={{ background: "var(--color-surface-secondary)", fontSize: "var(--text-label)", textTransform: "uppercase" }}>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Status</th>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Condition</th>
            </tr>
          </thead>
          <tbody>
            {m.statuses.map((s) => (
              <tr key={s.status} style={{ borderBottom: "1px solid var(--color-border)" }}>
                <td style={{ padding: "var(--space-sm)", fontWeight: 500 }}>{s.status}</td>
                <td style={{ padding: "var(--space-sm)", color: "var(--color-text-secondary)" }}>{s.condition}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>

      {/* Tie rule */}
      <section aria-labelledby="tie-heading">
        <h2 id="tie-heading" style={{ fontSize: "var(--text-h1)", fontWeight: 600, marginBottom: "var(--space-sm)" }}>Tie-Breaking Rule</h2>
        <p>{m.tie_rule}</p>
      </section>
    </article>
  );
}

export default function MethodologyPage() {
  return (
    <Suspense fallback={<SkeletonBlock variant="row" count={8} />}>
      <MethodologyContent />
    </Suspense>
  );
}
