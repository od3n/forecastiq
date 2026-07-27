"use client";

import { useMemo } from "react";
import { useApi } from "@/lib/api/hooks";
import { conditionIcon, conditionLabel } from "@/lib/conditions";

interface Observation {
  observed_at: string;
  value: number | null;
  condition_code?: string | null;
}

interface Snapshot {
  target_time: string;
  temperature_c: number | null;
  humidity_pct?: number | null;
  canonical_condition_code: string | null;
}

interface LatestForecastData {
  snapshots: Snapshot[];
}

interface ComparisonData {
  observations: Observation[];
}

interface Slot {
  offset: number; // hours relative to now (hour-truncated)
  kind: "observed" | "now" | "forecast";
  hourLabel: string;
  condition: string | null;
  temperature: number | null;
  humidity: number | null;
}

export interface ConditionsTimelineProps {
  providerId: string;
  locationId: string;
  /** IANA timezone of the location (hour labels). */
  timezone?: string;
  /** Omit the section heading (when the parent supplies its own labels). */
  hideHeading?: boolean;
}

// Operator context strip (admin): the last 12 observed hours, the current
// hour, and the next 12 forecast hours for one provider × location. Observed
// cells come from stored observations (ground truth); forecast cells from the
// provider's latest successful collection. Never shown on public screens
// (NP-01: ForecastIQ is not a weather product).
export function ConditionsTimeline({ providerId, locationId, timezone, hideHeading }: ConditionsTimelineProps) {
  // Dates (in the location timezone) covering the -12h window across midnight.
  const { todayStr, yesterdayStr } = useMemo(() => {
    const fmt = new Intl.DateTimeFormat("en-CA", { dateStyle: "short", ...(timezone ? { timeZone: timezone } : {}) });
    const now = new Date();
    const yesterday = new Date(now.getTime() - 24 * 3600 * 1000);
    return { todayStr: fmt.format(now), yesterdayStr: fmt.format(yesterday) };
  }, [timezone]);

  const obsPath = (d: string, variable: string) =>
    `/forecast-comparison?location_id=${locationId}&date=${d}&variable=${variable}&horizon_minutes=1440`;
  const { data: todayEnv } = useApi<ComparisonData>(locationId ? obsPath(todayStr, "temperature") : null);
  const { data: yesterdayEnv } = useApi<ComparisonData>(locationId ? obsPath(yesterdayStr, "temperature") : null);
  const { data: todayHumEnv } = useApi<ComparisonData>(locationId ? obsPath(todayStr, "humidity") : null);
  const { data: yesterdayHumEnv } = useApi<ComparisonData>(locationId ? obsPath(yesterdayStr, "humidity") : null);
  const { data: latestEnv } = useApi<LatestForecastData>(
    providerId && locationId ? `/forecasts/latest?provider_id=${providerId}&location_id=${locationId}` : null,
  );

  const slots: Slot[] = useMemo(() => {
    const hourMs = 3600 * 1000;
    const nowHour = Math.floor(Date.now() / hourMs) * hourMs;

    const obsByHour = new Map<number, Observation>();
    for (const o of [...(yesterdayEnv?.data?.observations ?? []), ...(todayEnv?.data?.observations ?? [])]) {
      obsByHour.set(new Date(o.observed_at).getTime(), o);
    }
    const humByHour = new Map<number, number>();
    for (const o of [...(yesterdayHumEnv?.data?.observations ?? []), ...(todayHumEnv?.data?.observations ?? [])]) {
      if (o.value !== null) humByHour.set(new Date(o.observed_at).getTime(), o.value);
    }
    const fcByHour = new Map<number, Snapshot>();
    for (const s of latestEnv?.data?.snapshots ?? []) {
      fcByHour.set(new Date(s.target_time).getTime(), s);
    }

    const hourFmt = new Intl.DateTimeFormat("en-GB", { hour: "2-digit", hour12: false, ...(timezone ? { timeZone: timezone } : {}) });
    const out: Slot[] = [];
    for (let offset = -12; offset <= 12; offset++) {
      const ts = nowHour + offset * hourMs;
      const obs = obsByHour.get(ts);
      const fc = fcByHour.get(ts);
      // Past + current prefer ground truth; future uses the forecast.
      const useObs = offset <= 0 && obs;
      out.push({
        offset,
        kind: offset === 0 ? "now" : offset < 0 ? "observed" : "forecast",
        hourLabel: hourFmt.format(new Date(ts)),
        condition: useObs ? (obs.condition_code ?? "unknown") : fc ? fc.canonical_condition_code : null,
        temperature: useObs ? obs.value : fc ? fc.temperature_c : null,
        humidity: useObs ? (humByHour.get(ts) ?? null) : fc ? (fc.humidity_pct ?? null) : null,
      });
    }
    return out;
  }, [todayEnv, yesterdayEnv, todayHumEnv, yesterdayHumEnv, latestEnv, timezone]);

  const hasAny = slots.some((s) => s.condition !== null || s.temperature !== null);
  if (!hasAny) {
    return <p style={{ fontSize: "var(--text-body-sm)", color: "var(--color-text-muted)", margin: 0 }}>No data in the ±12h window yet.</p>;
  }

  return (
    <section aria-labelledby={hideHeading ? undefined : "cond-timeline-heading"} style={{ marginBottom: hideHeading ? 0 : "var(--space-md)" }}>
      {!hideHeading && (
        <h2 id="cond-timeline-heading" style={{ fontSize: "var(--text-label)", textTransform: "uppercase", color: "var(--color-text-secondary)", marginBottom: "var(--space-xs)" }}>
          Now &plusmn;12h — observed (past) vs. forecast (ahead)
        </h2>
      )}
      <div style={{ display: "flex", gap: 2, overflowX: "auto", paddingBottom: "var(--space-xs)" }}>
        {slots.map((s) => (
          <div
            key={s.offset}
            title={`${s.hourLabel}:00 — ${s.kind === "forecast" ? "forecast" : "observed"}: ${s.condition ? conditionLabel(s.condition) : "no data"}${s.humidity !== null ? `, ${Math.round(s.humidity)}% humidity` : ""}${s.temperature !== null ? `, ${s.temperature}°C` : ""}`}
            style={{
              display: "flex", flexDirection: "column", alignItems: "center", minWidth: 40,
              padding: "var(--space-xs) 2px", borderRadius: "var(--radius-sm)",
              border: s.kind === "now" ? "2px solid var(--color-primary)" : "1px solid var(--color-border)",
              background: s.kind === "forecast" ? "var(--color-surface)" : "var(--color-surface-secondary)",
            }}
          >
            <span style={{ fontFamily: "var(--font-data)", fontSize: 10, color: s.kind === "now" ? "var(--color-primary)" : "var(--color-text-secondary)", fontWeight: s.kind === "now" ? 700 : 400 }}>
              {s.kind === "now" ? "Now" : s.hourLabel}
            </span>
            <span role="img" aria-label={s.condition ? conditionLabel(s.condition) : "no data"} style={{ fontSize: 18, lineHeight: 1.3 }}>
              {s.condition ? conditionIcon(s.condition) : "–"}
            </span>
            <span style={{ fontFamily: "var(--font-data)", fontSize: 10, fontWeight: 600, color: "var(--color-primary)" }}>
              {s.humidity !== null ? `${Math.round(s.humidity)}%` : ""}
            </span>
            <span style={{ fontFamily: "var(--font-data)", fontSize: 10, color: "var(--color-text-secondary)" }}>
              {s.temperature !== null ? `${Math.round(s.temperature)}°` : ""}
            </span>
          </div>
        ))}
      </div>
      {!hideHeading && (
        <p style={{ fontSize: "var(--text-label)", color: "var(--color-text-muted)", marginTop: 2 }}>
          Grey cells: stored observations (ground truth). White cells: the provider&rsquo;s latest forecast. Operator context only.
        </p>
      )}
    </section>
  );
}
