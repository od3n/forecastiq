"use client";

import { Suspense } from "react";
import { useApi } from "@/lib/api/hooks";
import { SkeletonBlock } from "@/components/SkeletonBlock";
import { ErrorPanel } from "@/components/ErrorPanel";

interface MethodologyData {
  methodology_version: string;
  weights_version: string;
  composite_formula: string;
  composite_description: string;
  weights: Record<string, number>;
  thresholds: {
    min_sample_count: number;
    provisional_range: [number, number];
    coverage_penalty_threshold: number;
  };
  ranking_statuses: { status: string; description: string }[];
  tie_rule: string;
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

      {/* Composite formula */}
      <section aria-labelledby="formula-heading" style={{ marginBottom: "var(--space-xl)" }}>
        <h2 id="formula-heading" style={{ fontSize: "var(--text-h1)", fontWeight: 600, marginBottom: "var(--space-sm)" }}>Composite Score Formula</h2>
        <pre style={{ fontFamily: "var(--font-data)", background: "var(--color-surface-secondary)", padding: "var(--space-md)", borderRadius: "var(--radius-md)", overflowX: "auto" }}>
          {m.composite_formula}
        </pre>
        <p style={{ color: "var(--color-text-secondary)", marginTop: "var(--space-sm)" }}>
          {m.composite_description}
        </p>
      </section>

      {/* Weights */}
      <section aria-labelledby="weights-heading" style={{ marginBottom: "var(--space-xl)" }}>
        <h2 id="weights-heading" style={{ fontSize: "var(--text-h1)", fontWeight: 600, marginBottom: "var(--space-sm)" }}>Component Weights</h2>
        <table style={{ width: "100%", borderCollapse: "collapse" }}>
          <thead>
            <tr style={{ background: "var(--color-surface-secondary)", fontSize: "var(--text-label)", textTransform: "uppercase" }}>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Component</th>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "right" }}>Weight</th>
            </tr>
          </thead>
          <tbody>
            {Object.entries(m.weights).map(([name, weight]) => (
              <tr key={name} style={{ borderBottom: "1px solid var(--color-border)" }}>
                <td style={{ padding: "var(--space-sm)" }}>{name}</td>
                <td style={{ padding: "var(--space-sm)", textAlign: "right", fontFamily: "var(--font-data)" }}>{weight}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>

      {/* Thresholds */}
      <section aria-labelledby="thresholds-heading" style={{ marginBottom: "var(--space-xl)" }}>
        <h2 id="thresholds-heading" style={{ fontSize: "var(--text-h1)", fontWeight: 600, marginBottom: "var(--space-sm)" }}>Thresholds</h2>
        <dl style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "var(--space-sm)" }}>
          <dt style={{ color: "var(--color-text-secondary)" }}>Minimum sample count</dt>
          <dd style={{ fontFamily: "var(--font-data)", margin: 0 }}>{m.thresholds.min_sample_count}</dd>
          <dt style={{ color: "var(--color-text-secondary)" }}>Provisional range (samples)</dt>
          <dd style={{ fontFamily: "var(--font-data)", margin: 0 }}>{m.thresholds.provisional_range[0]}–{m.thresholds.provisional_range[1]}</dd>
          <dt style={{ color: "var(--color-text-secondary)" }}>Coverage penalty threshold</dt>
          <dd style={{ fontFamily: "var(--font-data)", margin: 0 }}>{(m.thresholds.coverage_penalty_threshold * 100).toFixed(0)}%</dd>
        </dl>
      </section>

      {/* Ranking statuses */}
      <section aria-labelledby="statuses-heading" style={{ marginBottom: "var(--space-xl)" }}>
        <h2 id="statuses-heading" style={{ fontSize: "var(--text-h1)", fontWeight: 600, marginBottom: "var(--space-sm)" }}>Ranking Statuses</h2>
        <table style={{ width: "100%", borderCollapse: "collapse" }}>
          <thead>
            <tr style={{ background: "var(--color-surface-secondary)", fontSize: "var(--text-label)", textTransform: "uppercase" }}>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Status</th>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Description</th>
            </tr>
          </thead>
          <tbody>
            {m.ranking_statuses.map((s) => (
              <tr key={s.status} style={{ borderBottom: "1px solid var(--color-border)" }}>
                <td style={{ padding: "var(--space-sm)", fontWeight: 500 }}>{s.status}</td>
                <td style={{ padding: "var(--space-sm)", color: "var(--color-text-secondary)" }}>{s.description}</td>
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
