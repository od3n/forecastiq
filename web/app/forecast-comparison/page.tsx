"use client";

import { Suspense, useMemo } from "react";
import { useSearchParams, useRouter, usePathname } from "next/navigation";
import { useGlobalParams } from "@/lib/state/use-global-params";
import { useApi } from "@/lib/api/hooks";
import { OverlayChart } from "@/components/OverlayChart";
import { DayMetricsTable, type DayMetric } from "@/components/DayMetricsTable";
import type { ChartDataPoint } from "@/components/ChartDataTable";
import { VariableSelector } from "@/components/VariableSelector";
import { ExportButton } from "@/components/ExportButton";
import { SkeletonBlock } from "@/components/SkeletonBlock";
import { ErrorPanel } from "@/components/ErrorPanel";
import { StaleBanner } from "@/components/StaleBanner";
import { PartialWarnings } from "@/components/PartialWarnings";
import { AttributionFooter } from "@/components/AttributionFooter";
import type { Freshness, Warning, Attribution } from "@/lib/api/types";
import type { CsvExportInput } from "@/lib/csv/export";
import { conditionIcon, conditionLabel } from "@/lib/conditions";

interface FvaPoint {
  target_time: string;
  value: number | null;
}

interface FvaSeries {
  provider: { id: string; name: string; slug: string };
  issued_at: string;
  points: FvaPoint[];
}

interface FvaObservation {
  observed_at: string;
  value: number | null;
  source: string;
  quality_flag: string;
  condition_code?: string | null;
}

interface FvaDayMetric {
  provider_id: string;
  mae: number | null;
  rmse: number | null;
  bias: number | null;
  sample_count: number;
}

interface FvaData {
  location?: { id: string; name: string; timezone: string };
  series: FvaSeries[];
  observations: FvaObservation[];
  day_metrics: FvaDayMetric[];
  observations_available: boolean;
}

const UNITS: Record<string, string> = { temperature: "°C", precipitation: "mm", wind_speed: "m/s", humidity: "%", pressure: "hPa" };

/** Today's date (YYYY-MM-DD) in the browser's local timezone — toISOString()
 * would give the UTC date, which lags behind local time (e.g. Malaysia
 * mornings would default to yesterday). */
function today(): string {
  return new Date().toLocaleDateString("en-CA");
}

function FvaContent() {
  const { locationId, horizonMinutes } = useGlobalParams();
  const searchParams = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();

  const date = searchParams.get("date") ?? today();
  const variable = searchParams.get("variable") ?? "temperature";
  const unit = UNITS[variable] ?? "";

  const setParam = (key: string, value: string) => {
    const params = new URLSearchParams(searchParams.toString());
    params.set(key, value);
    router.replace(`${pathname}?${params.toString()}`, { scroll: false });
  };

  const path = locationId
    ? `/forecast-comparison?location_id=${locationId}&date=${date}&variable=${variable}&horizon_minutes=${horizonMinutes}`
    : null;

  const { data: envelope, error, isLoading } = useApi<FvaData>(path);

  // Transform API series/observations into a fixed 24-hour day (location
  // timezone). Every hour slot exists even without data, so the x-axis always
  // spans 00:00–23:00 of the selected date.
  const { chartData, providers, dayMetrics, obsAvailable, conditionByHour, hasData } = useMemo(() => {
    const series = envelope?.data?.series ?? [];
    const observations = envelope?.data?.observations ?? [];
    const rawMetrics = envelope?.data?.day_metrics ?? [];
    const obsOk = envelope?.data?.observations_available ?? true;
    const tz = envelope?.data?.location?.timezone;

    const provs = series.map((s) => s.provider.name);
    const providerNameByID = new Map(series.map((s) => [s.provider.id, s.provider.name]));

    // Fixed 24 hour slots keyed "00".."23" (location-tz hour of each timestamp).
    const hourFmt = new Intl.DateTimeFormat("en-GB", { hour: "2-digit", hour12: false, ...(tz ? { timeZone: tz } : {}) });
    const hourOf = (iso: string) => hourFmt.format(new Date(iso)).padStart(2, "0");

    const slots = new Map<string, ChartDataPoint>();
    for (let h = 0; h < 24; h++) {
      const key = String(h).padStart(2, "0");
      slots.set(key, { period_start: `${key}:00`, observation: null });
    }
    const conditions = new Map<string, string>();
    let any = false;
    for (const o of observations) {
      const row = slots.get(hourOf(o.observed_at));
      if (!row) continue;
      row["observation"] = o.value;
      conditions.set(hourOf(o.observed_at), o.condition_code ?? "unknown");
      any = true;
    }
    for (const s of series) {
      for (const p of s.points) {
        const row = slots.get(hourOf(p.target_time));
        if (!row) continue;
        row[s.provider.name] = p.value;
        if (p.value !== null) any = true;
      }
    }
    const chartData = Array.from(slots.values());

    const dm: DayMetric[] = rawMetrics.map((m) => ({
      provider_name: providerNameByID.get(m.provider_id) ?? m.provider_id,
      mae: m.mae, rmse: m.rmse, bias: m.bias,
    }));

    return { chartData, providers: provs, dayMetrics: dm, obsAvailable: obsOk, conditionByHour: conditions, hasData: any };
  }, [envelope]);

  const csvInput: CsvExportInput | null = useMemo(() => {
    if (!hasData) return null;
    return {
      screen: "Forecast vs. Actual (S-05)",
      variable,
      columns: ["time", "observation", ...providers],
      rows: chartData.map((r) => [r.period_start, r["observation"] ?? null, ...providers.map((p) => r[p] ?? null)]),
      generatedAt: new Date().toISOString(),
      attribution: envelope?.attribution?.map((a) => ({ provider: a.provider, url: a.url })),
    };
  }, [chartData, hasData, providers, variable, envelope]);

  if (isLoading) {
    return (
      <section aria-labelledby="fva-heading" aria-busy="true">
        <h1 id="fva-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Forecast vs. Actual</h1>
        <SkeletonBlock variant="chart" />
      </section>
    );
  }

  if (error) {
    return (
      <section aria-labelledby="fva-heading">
        <h1 id="fva-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Forecast vs. Actual</h1>
        <ErrorPanel message="Unable to load comparison data." requestId={error.requestId} onRetry={() => window.location.reload()} />
      </section>
    );
  }

  const freshness = envelope?.freshness as Freshness | undefined;
  const warnings = envelope?.warnings as Warning[] | undefined;
  const attribution = envelope?.attribution as Attribution[] | undefined;
  const methodology = envelope?.metadata?.methodology_version;

  return (
    <section aria-labelledby="fva-heading">
      <h1 id="fva-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Forecast vs. Actual</h1>

      {/* Controls */}
      <div style={{ display: "flex", flexWrap: "wrap", gap: "var(--space-sm)", alignItems: "center", marginBottom: "var(--space-md)" }}>
        <label style={{ fontSize: "var(--text-label)", color: "var(--color-text-secondary)" }}>
          Date:
          <input type="date" value={date} onChange={(e) => setParam("date", e.target.value)} style={{ marginLeft: "var(--space-xs)", padding: "var(--space-xs) var(--space-sm)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", font: "inherit", fontSize: "var(--text-body-sm)" }} />
        </label>
        <VariableSelector selected={variable} onChange={(v) => setParam("variable", v)} />
        <span style={{ flex: 1 }} />
        <ExportButton exportInput={csvInput} filename={`forecastiq-fva-${date}-${variable}`} />
      </div>

      {freshness?.state === "stale" && freshness.last_updated && (
        <div style={{ marginBottom: "var(--space-md)" }}><StaleBanner lastUpdated={freshness.last_updated} /></div>
      )}
      {warnings && warnings.length > 0 && (
        <div style={{ marginBottom: "var(--space-md)" }}><PartialWarnings warnings={warnings} /></div>
      )}
      {!obsAvailable && (
        <div style={{ marginBottom: "var(--space-md)", padding: "var(--space-sm) var(--space-md)", background: "var(--color-surface-secondary)", borderRadius: "var(--radius-md)", fontSize: "var(--text-body-sm)", color: "var(--color-text-secondary)" }}>
          Ground truth unavailable for this period — metrics not computed. Forecast lines shown without observation overlay.
        </div>
      )}

      <p style={{ fontSize: "var(--text-body-sm)", color: "var(--color-text-secondary)", marginBottom: "var(--space-sm)" }}>
        Forecasts issued for {date} · Horizon +{horizonMinutes / 60}h
      </p>

      {!hasData ? (
        <div style={{ height: 400, display: "flex", alignItems: "center", justifyContent: "center", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", color: "var(--color-text-secondary)" }}>
          <p>No forecasts collected for {date}. Try a different date.</p>
        </div>
      ) : (
        <>
          <OverlayChart data={chartData} providers={providers} unit={unit} />

          {/* Observed conditions — bordered 24-column table aligned with the
              chart plot area (left offset ≈ y-axis 60px + left margin 8px;
              right offset = chart right margin 16px). */}
          <section aria-labelledby="obs-cond-heading" style={{ marginTop: "var(--space-md)" }}>
            <h2 id="obs-cond-heading" style={{ fontSize: "var(--text-label)", textTransform: "uppercase", color: "var(--color-text-secondary)", marginBottom: "var(--space-xs)" }}>
              Observed conditions
            </h2>
            <div style={{ marginLeft: 68, marginRight: 16, overflowX: "auto" }}>
              <table style={{ width: "100%", tableLayout: "fixed", borderCollapse: "collapse" }}>
                <thead>
                  <tr>
                    {chartData.map((r) => (
                      <th key={r.period_start} scope="col" style={{ border: "1px solid var(--color-border)", padding: "2px 0", fontFamily: "var(--font-data)", fontSize: 9, fontWeight: 400, color: "var(--color-text-secondary)", textAlign: "center" }}>
                        {String(r.period_start).slice(0, 2)}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  <tr>
                    {chartData.map((r) => {
                      const hour = String(r.period_start).slice(0, 2);
                      const cond = conditionByHour.get(hour);
                      return (
                        <td
                          key={hour}
                          title={cond ? `${hour}:00 — ${cond.replace(/_/g, " ")}` : `${hour}:00 — no observation`}
                          style={{ border: "1px solid var(--color-border)", textAlign: "center", padding: "4px 0", fontSize: 15, lineHeight: 1, background: cond ? "var(--color-surface)" : "var(--color-surface-secondary)" }}
                        >
                          {cond ? (
                            <span role="img" aria-label={conditionLabel(cond)}>{conditionIcon(cond)}</span>
                          ) : (
                            <span aria-label="no observation" style={{ color: "var(--color-text-muted)" }}>–</span>
                          )}
                        </td>
                      );
                    })}
                  </tr>
                </tbody>
              </table>
            </div>
          </section>

          <DayMetricsTable metrics={dayMetrics} unit={unit} />
        </>
      )}

      {attribution && (
        <div style={{ marginTop: "var(--space-xl)" }}><AttributionFooter providers={attribution} methodologyVersion={methodology} /></div>
      )}
    </section>
  );
}

export default function ForecastComparisonPage() {
  return (
    <Suspense fallback={<SkeletonBlock variant="chart" />}>
      <FvaContent />
    </Suspense>
  );
}
