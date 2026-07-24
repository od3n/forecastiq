"use client";

import { useState, useCallback } from "react";
import {
  ResponsiveContainer,
  LineChart,
  Line,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
} from "recharts";
import { ChartDataTable, ChartWrapper, type ChartDataPoint } from "./ChartDataTable";
import styles from "./TrendChart.module.css"; // reuse chart styles

const PROVIDER_COLORS: Record<string, string> = {
  "Open-Meteo": "#2563eb",
  "OpenWeather": "#7c3aed",
};
function getColor(provider: string, idx: number): string {
  return PROVIDER_COLORS[provider] ?? ["#0891b2", "#db2777"][idx % 2] ?? "#6b7280";
}

export interface OverlayChartProps {
  /** Hourly data points for the selected day. */
  data: ChartDataPoint[];
  /** Provider names for forecast lines. */
  providers: string[];
  /** Unit label (e.g. "°C"). */
  unit: string;
  /** Error band upper/lower keys (around observation). */
  errorBand?: { upper: string; lower: string };
}

// S-05 Forecast vs. Actual chart (doc 02 §5.3). Observation line is gray-900
// dashed 2.5px; forecast lines are 2px solid provider-colored. Error band is
// +/- MAE around observation (10% opacity). Missing observations break the line.
// Keyboard nav + legend toggles + hidden data table (same a11y as TrendChart).
export function OverlayChart({ data, providers, unit, errorBand }: OverlayChartProps) {
  const [hidden, setHidden] = useState<Set<string>>(new Set());
  const [focusIdx, setFocusIdx] = useState(0);

  const toggle = useCallback((key: string) => {
    setHidden((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key); else next.add(key);
      return next;
    });
  }, []);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === "ArrowRight") { e.preventDefault(); setFocusIdx((i) => Math.min(i + 1, data.length - 1)); }
    else if (e.key === "ArrowLeft") { e.preventDefault(); setFocusIdx((i) => Math.max(i - 1, 0)); }
  }, [data.length]);

  const focused = data[focusIdx];
  const announcement = focused
    ? `${focused.period_start}: observed ${focused["observation"] ?? "no data"}, ${providers.filter((p) => !hidden.has(p)).map((p) => `${p} ${focused[p] ?? "no data"}`).join(", ")} ${unit}`
    : "";

  const allKeys = ["observation", ...providers];

  return (
    <div>
      <ChartWrapper label={`Forecast vs. Actual chart (${unit})`}>
        <div className={styles.chartContainer} tabIndex={0} onKeyDown={handleKeyDown} aria-label="Chart. Use left/right arrows to navigate hours.">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={data} margin={{ top: 8, right: 16, bottom: 24, left: 8 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
              <XAxis dataKey="period_start" tick={{ fontSize: 11 }} />
              <YAxis unit={` ${unit}`} tick={{ fontSize: 11 }} />
              <Tooltip />
              {/* Error band around observation */}
              {errorBand && !hidden.has("observation") && (
                <Area dataKey={errorBand.upper} stroke="none" fill="#111827" fillOpacity={0.08} baseValue="dataMin" isAnimationActive={false} connectNulls={false} />
              )}
              {/* Observation line: dashed, 2.5px, gray-900 */}
              {!hidden.has("observation") && (
                <Line dataKey="observation" stroke="#111827" strokeWidth={2.5} strokeDasharray="6 3" dot={false} connectNulls={false} isAnimationActive={false} />
              )}
              {/* Forecast lines: 2px solid, provider-colored */}
              {providers.filter((p) => !hidden.has(p)).map((p, i) => (
                <Line key={p} dataKey={p} stroke={getColor(p, i)} strokeWidth={2} dot={{ r: 2 }} connectNulls={false} isAnimationActive={false} />
              ))}
            </LineChart>
          </ResponsiveContainer>
        </div>
        <ChartDataTable data={data} providers={allKeys} unit={unit} />
        <div className={styles.announcement} aria-live="polite" aria-atomic="true">{announcement}</div>
      </ChartWrapper>
      {/* Legend */}
      <div className={styles.legend}>
        <button type="button" className={`${styles.legendBtn} ${hidden.has("observation") ? styles.legendHidden : ""}`} onClick={() => toggle("observation")} aria-pressed={!hidden.has("observation")} aria-label={`${hidden.has("observation") ? "Show" : "Hide"} Observation`}>
          <span className={styles.legendDot} style={{ background: "#111827", borderRadius: 0, width: 14, height: 3 }} />
          Observed
        </button>
        {providers.map((p, i) => (
          <button key={p} type="button" className={`${styles.legendBtn} ${hidden.has(p) ? styles.legendHidden : ""}`} onClick={() => toggle(p)} aria-pressed={!hidden.has(p)} aria-label={`${hidden.has(p) ? "Show" : "Hide"} ${p}`}>
            <span className={styles.legendDot} style={{ background: getColor(p, i) }} />
            {p}
          </button>
        ))}
      </div>
    </div>
  );
}
