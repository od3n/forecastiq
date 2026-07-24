"use client";

import { useState, useRef, useCallback } from "react";
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
import styles from "./TrendChart.module.css";

// Provider color mapping (matching design tokens in globals.css).
const PROVIDER_COLORS: Record<string, string> = {
  "Open-Meteo": "#2563eb",
  "OpenWeather": "#7c3aed",
  default1: "#0891b2",
  default2: "#db2777",
};

function getColor(provider: string, idx: number): string {
  return PROVIDER_COLORS[provider] ?? Object.values(PROVIDER_COLORS)[idx % Object.values(PROVIDER_COLORS).length] ?? "#6b7280";
}

export interface TrendSeries {
  provider: string;
  color?: string;
}

export interface TrendChartProps {
  data: ChartDataPoint[];
  series: TrendSeries[];
  unit: string;
  /** CI band keys per provider: { "Open-Meteo": { lower: "om_ci_lower", upper: "om_ci_upper" } } */
  ciBands?: Record<string, { lower: string; upper: string }>;
  /** Sample-count key per provider for hollow-dot detection */
  sampleKeys?: Record<string, string>;
  sampleThreshold?: number;
}

// Custom dot: hollow when sample_count < threshold (provisional data).
function HollowDot(props: { cx?: number; cy?: number; fill?: string; stroke?: string; payload?: ChartDataPoint; sampleKey?: string; threshold?: number }) {
  const { cx = 0, cy = 0, stroke, payload, sampleKey, threshold = 30 } = props;
  if (!payload || !sampleKey) return <circle cx={cx} cy={cy} r={3} fill={stroke} stroke={stroke} />;
  const count = Number(payload[sampleKey]) || 0;
  if (count < threshold) {
    return <circle cx={cx} cy={cy} r={4} fill="transparent" stroke={stroke} strokeWidth={2} />;
  }
  return <circle cx={cx} cy={cy} r={3} fill={stroke} stroke={stroke} />;
}

// TrendChart (S-04; doc 02 §5.3). Recharts-based with:
// - Provider-colored lines (2px stroke)
// - CI bands (Area, 10% opacity)
// - Line breaks for gaps (null values)
// - Hollow dots for provisional data
// - Keyboard navigation (arrow keys move focus indicator, aria-live announces)
// - Legend toggles (focusable buttons)
// - Data-table fallback for screen readers
export function TrendChart({ data, series, unit, ciBands, sampleKeys, sampleThreshold = 30 }: TrendChartProps) {
  const [hidden, setHidden] = useState<Set<string>>(new Set());
  const [focusIdx, setFocusIdx] = useState(0);
  const announcementRef = useRef<HTMLDivElement>(null);

  const toggle = useCallback((provider: string) => {
    setHidden((prev) => {
      const next = new Set(prev);
      if (next.has(provider)) next.delete(provider);
      else next.add(provider);
      return next;
    });
  }, []);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === "ArrowRight") {
        e.preventDefault();
        setFocusIdx((i) => Math.min(i + 1, data.length - 1));
      } else if (e.key === "ArrowLeft") {
        e.preventDefault();
        setFocusIdx((i) => Math.max(i - 1, 0));
      }
    },
    [data.length],
  );

  // Build announcement text for the focused data point.
  const focusedPoint = data[focusIdx];
  const announcement = focusedPoint
    ? `${focusedPoint.period_start}: ${series.filter((s) => !hidden.has(s.provider)).map((s) => `${s.provider} ${focusedPoint[s.provider] ?? "no data"}`).join(", ")} ${unit}`
    : "";

  const providerNames = series.map((s) => s.provider);

  return (
    <div>
      <ChartWrapper label={`Accuracy trends chart (${unit})`}>
        <div
          className={styles.chartContainer}
          tabIndex={0}
          onKeyDown={handleKeyDown}
          aria-label="Chart. Use left/right arrow keys to navigate data points."
        >
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={data} margin={{ top: 8, right: 16, bottom: 24, left: 8 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
              <XAxis dataKey="period_start" tick={{ fontSize: 11 }} />
              <YAxis unit={` ${unit}`} tick={{ fontSize: 11 }} />
              <Tooltip />
              {/* CI bands */}
              {ciBands &&
                series
                  .filter((s) => !hidden.has(s.provider))
                  .map((s, i) => {
                    const band = ciBands[s.provider];
                    if (!band) return null;
                    const color = s.color ?? getColor(s.provider, i);
                    return (
                      <Area
                        key={`ci-${s.provider}`}
                        dataKey={band.upper}
                        stroke="none"
                        fill={color}
                        fillOpacity={0.1}
                        baseValue="dataMin"
                        isAnimationActive={false}
                        connectNulls={false}
                      />
                    );
                  })}
              {/* Lines */}
              {series
                .filter((s) => !hidden.has(s.provider))
                .map((s, i) => {
                  const color = s.color ?? getColor(s.provider, i);
                  return (
                    <Line
                      key={s.provider}
                      dataKey={s.provider}
                      stroke={color}
                      strokeWidth={2}
                      dot={<HollowDot stroke={color} sampleKey={sampleKeys?.[s.provider]} threshold={sampleThreshold} />}
                      connectNulls={false}
                      isAnimationActive={false}
                    />
                  );
                })}
            </LineChart>
          </ResponsiveContainer>
        </div>
        {/* Hidden data table for screen readers */}
        <ChartDataTable data={data} providers={providerNames} unit={unit} />
        {/* Keyboard nav announcement */}
        <div ref={announcementRef} className={styles.announcement} aria-live="polite" aria-atomic="true">
          {announcement}
        </div>
      </ChartWrapper>
      {/* Legend (focusable buttons to toggle series; doc 07 §2 S-04) */}
      <div className={styles.legend}>
        {series.map((s, i) => {
          const color = s.color ?? getColor(s.provider, i);
          const isHidden = hidden.has(s.provider);
          return (
            <button
              key={s.provider}
              type="button"
              className={`${styles.legendBtn} ${isHidden ? styles.legendHidden : ""}`}
              onClick={() => toggle(s.provider)}
              aria-pressed={!isHidden}
              aria-label={`${isHidden ? "Show" : "Hide"} ${s.provider}`}
            >
              <span className={styles.legendDot} style={{ background: color }} />
              {s.provider}
            </button>
          );
        })}
      </div>
    </div>
  );
}
