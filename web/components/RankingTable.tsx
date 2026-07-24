"use client";

import { useState } from "react";
import { StatusBadge, type RankingStatus } from "./StatusBadge";
import { FreshnessBadge } from "./FreshnessBadge";
import type { FreshnessState } from "@/lib/api/types";

export interface RankingEntry {
  rank: number | null;
  provider_id: string;
  provider_name: string;
  composite_score: number | null;
  ranking_status: RankingStatus;
  sample_count: number;
  coverage: number | null;
  components?: Record<string, number | null>;
  penalty_applied?: boolean;
}

export interface RankingTableProps {
  rankings: RankingEntry[];
  freshness?: { state: FreshnessState; last_updated?: string };
  methodologyVersion?: string;
}

// Ranking table (S-01; doc 02 §4). Semantic <table> with expandable rows for
// component breakdown; StatusBadge per provider (ranked/provisional/unranked);
// sample count highlighted below threshold; tab/Enter/Space for expand (a11y §2 S-01).
export function RankingTable({ rankings, freshness, methodologyVersion }: RankingTableProps) {
  const [expanded, setExpanded] = useState<string | null>(null);

  const toggle = (id: string) => setExpanded((prev) => (prev === id ? null : id));

  return (
    <div>
      {freshness && (
        <div style={{ marginBottom: "var(--space-sm)", display: "flex", alignItems: "center", gap: "var(--space-sm)" }}>
          <FreshnessBadge state={freshness.state} lastUpdated={freshness.last_updated} />
          {methodologyVersion && (
            <span style={{ fontSize: "var(--text-label)", color: "var(--color-text-muted)" }}>
              Methodology {methodologyVersion}
            </span>
          )}
        </div>
      )}
      <table style={{ width: "100%", borderCollapse: "collapse" }}>
        <thead>
          <tr style={{ background: "var(--color-surface-secondary)", fontSize: "var(--text-label)", textTransform: "uppercase" as const }}>
            <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>#</th>
            <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Provider</th>
            <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "right" }}>Score</th>
            <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Status</th>
            <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "right" }}>Samples</th>
          </tr>
        </thead>
        <tbody>
          {rankings.map((r) => (
            <RankingRow
              key={r.provider_id}
              entry={r}
              isExpanded={expanded === r.provider_id}
              onToggle={() => toggle(r.provider_id)}
            />
          ))}
        </tbody>
      </table>
    </div>
  );
}

function RankingRow({ entry: r, isExpanded, onToggle }: { entry: RankingEntry; isExpanded: boolean; onToggle: () => void }) {
  const sampleBelowThreshold = r.sample_count < 30;
  return (
    <>
      <tr
        style={{ borderBottom: "1px solid var(--color-border)", cursor: "pointer", height: 44 }}
        onClick={onToggle}
        onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onToggle(); } }}
        tabIndex={0}
        aria-expanded={isExpanded}
      >
        <td style={{ padding: "var(--space-sm)", fontFamily: "var(--font-data)", fontWeight: 500 }}>
          {r.rank ?? "—"}
        </td>
        <td style={{ padding: "var(--space-sm)" }}>{r.provider_name}</td>
        <td style={{ padding: "var(--space-sm)", textAlign: "right", fontFamily: "var(--font-data)" }}>
          {r.composite_score !== null ? r.composite_score.toFixed(3) : "—"}
        </td>
        <td style={{ padding: "var(--space-sm)" }}>
          <StatusBadge status={r.ranking_status} sampleCount={r.sample_count} />
        </td>
        <td style={{ padding: "var(--space-sm)", textAlign: "right", fontFamily: "var(--font-data)", color: sampleBelowThreshold ? "var(--color-delayed)" : undefined, fontWeight: sampleBelowThreshold ? 600 : undefined }}>
          {r.sample_count}
        </td>
      </tr>
      {isExpanded && r.components && (
        <tr>
          <td colSpan={5} role="region" aria-label={`Breakdown for ${r.provider_name}`}>
            <div style={{ padding: "var(--space-sm) var(--space-md)", background: "var(--color-surface-secondary)", fontSize: "var(--text-body-sm)" }}>
              <strong>Component breakdown</strong>
              <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(140px, 1fr))", gap: "var(--space-sm)", marginTop: "var(--space-xs)" }}>
                {Object.entries(r.components).map(([name, val]) => (
                  <div key={name}>
                    <span style={{ color: "var(--color-text-secondary)" }}>{name}: </span>
                    <span style={{ fontFamily: "var(--font-data)" }}>{val !== null ? val.toFixed(4) : "—"}</span>
                  </div>
                ))}
              </div>
              {r.penalty_applied && (
                <p style={{ color: "var(--color-delayed)", marginTop: "var(--space-xs)" }}>
                  Score penalized for incomplete data coverage.
                </p>
              )}
            </div>
          </td>
        </tr>
      )}
    </>
  );
}
