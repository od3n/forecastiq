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
import { EmptyState } from "@/components/EmptyState";
import { ErrorPanel } from "@/components/ErrorPanel";
import { StaleBanner } from "@/components/StaleBanner";
import { PartialWarnings } from "@/components/PartialWarnings";
import { AttributionFooter } from "@/components/AttributionFooter";
import type { Freshness, Warning, Attribution } from "@/lib/api/types";
import type { CsvExportInput } from "@/lib/csv/export";

interface FvaHour {
  time: string;
  observation: number | null;
  forecasts: Record<string, number | null>;
}

interface FvaData {
  hours: FvaHour[];
  providers: string[];
  day_metrics: DayMetric[];
  observations_available: boolean;
}

const UNITS: Record<string, string> = { temperature: "°C", precipitation: "mm", wind_speed: "m/s", humidity: "%", pressure: "hPa" };

function today(): string {
  return new Date().toISOString().slice(0, 10);
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

  // Transform to chart data points.
  const { chartData, providers, dayMetrics, obsAvailable } = useMemo(() => {
    const hours = envelope?.data?.hours ?? [];
    const provs = envelope?.data?.providers ?? [];
    const dm = envelope?.data?.day_metrics ?? [];
    const obsOk = envelope?.data?.observations_available ?? true;
    const chartData: ChartDataPoint[] = hours.map((h) => {
      const pt: ChartDataPoint = { period_start: h.time, observation: h.observation };
      for (const p of provs) pt[p] = h.forecasts[p] ?? null;
      return pt;
    });
    return { chartData, providers: provs, dayMetrics: dm, obsAvailable: obsOk };
  }, [envelope]);

  const csvInput: CsvExportInput | null = useMemo(() => {
    if (chartData.length === 0) return null;
    return {
      screen: "Forecast vs. Actual (S-05)",
      variable,
      columns: ["time", "observation", ...providers],
      rows: chartData.map((r) => [r.period_start, r["observation"] ?? null, ...providers.map((p) => r[p] ?? null)]),
      generatedAt: new Date().toISOString(),
      attribution: envelope?.attribution?.map((a) => ({ provider: a.provider, url: a.url })),
    };
  }, [chartData, providers, variable, envelope]);

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

  if (chartData.length === 0) {
    return (
      <section aria-labelledby="fva-heading">
        <h1 id="fva-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Forecast vs. Actual</h1>
        <EmptyState variant="no-data" title="No forecasts for this date" description={`No forecasts collected for ${date}. Try a different date.`} />
      </section>
    );
  }

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

      <OverlayChart data={chartData} providers={providers} unit={unit} />

      <DayMetricsTable metrics={dayMetrics} unit={unit} />

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
