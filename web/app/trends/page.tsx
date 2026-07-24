"use client";

import { Suspense, useMemo } from "react";
import { useSearchParams, useRouter, usePathname } from "next/navigation";
import { useGlobalParams } from "@/lib/state/use-global-params";
import { useApi } from "@/lib/api/hooks";
import { TrendChart, type TrendSeries } from "@/components/TrendChart";
import type { ChartDataPoint } from "@/components/ChartDataTable";
import { VariableSelector } from "@/components/VariableSelector";
import { AggregationSelector } from "@/components/AggregationSelector";
import { DateRangePicker } from "@/components/DateRangePicker";
import { ExportButton } from "@/components/ExportButton";
import { SkeletonBlock } from "@/components/SkeletonBlock";
import { EmptyState } from "@/components/EmptyState";
import { ErrorPanel } from "@/components/ErrorPanel";
import { StaleBanner } from "@/components/StaleBanner";
import { PartialWarnings } from "@/components/PartialWarnings";
import { AttributionFooter } from "@/components/AttributionFooter";
import type { Freshness, Warning, Attribution } from "@/lib/api/types";
import type { CsvExportInput } from "@/lib/csv/export";

interface TrendBucket {
  period_start: string;
  provider_name: string;
  value: number | null;
  ci_lower: number | null;
  ci_upper: number | null;
  sample_count: number;
}

interface TrendsData {
  metrics: TrendBucket[];
  providers: string[];
}

const UNITS: Record<string, string> = {
  temperature: "°C",
  precipitation: "mm",
  wind_speed: "m/s",
  humidity: "%",
  pressure: "hPa",
};

function TrendsContent() {
  const { locationId, horizonMinutes } = useGlobalParams();
  const searchParams = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();

  const variable = searchParams.get("variable") ?? "temperature";
  const aggregation = searchParams.get("aggregation") ?? "daily";
  const period = searchParams.get("period") ?? "30d";
  const unit = UNITS[variable] ?? "";

  const setParam = (key: string, value: string) => {
    const params = new URLSearchParams(searchParams.toString());
    params.set(key, value);
    router.replace(`${pathname}?${params.toString()}`, { scroll: false });
  };

  const path = locationId
    ? `/accuracy?location_id=${locationId}&variable=${variable}&aggregation=${aggregation}&horizon_minutes=${horizonMinutes}&period=${period}`
    : null;

  const { data: envelope, error, isLoading } = useApi<TrendsData>(path);

  // Transform API buckets into Recharts-friendly data points.
  const { chartData, series, ciBands, sampleKeys } = useMemo(() => {
    const metrics = envelope?.data?.metrics ?? [];
    const providers = envelope?.data?.providers ?? [];
    if (metrics.length === 0) return { chartData: [] as ChartDataPoint[], series: [] as TrendSeries[], ciBands: {}, sampleKeys: {} };

    // Group by period_start.
    const byPeriod = new Map<string, ChartDataPoint>();
    for (const m of metrics) {
      if (!byPeriod.has(m.period_start)) byPeriod.set(m.period_start, { period_start: m.period_start });
      const row = byPeriod.get(m.period_start)!;
      row[m.provider_name] = m.value;
      row[`${m.provider_name}_ci_upper`] = m.ci_upper;
      row[`${m.provider_name}_ci_lower`] = m.ci_lower;
      row[`${m.provider_name}_samples`] = m.sample_count;
    }

    const chartData = Array.from(byPeriod.values()).sort((a, b) => a.period_start.localeCompare(b.period_start));
    const seriesList: TrendSeries[] = providers.map((p) => ({ provider: p }));
    const bands: Record<string, { lower: string; upper: string }> = {};
    const samples: Record<string, string> = {};
    for (const p of providers) {
      bands[p] = { lower: `${p}_ci_lower`, upper: `${p}_ci_upper` };
      samples[p] = `${p}_samples`;
    }

    return { chartData, series: seriesList, ciBands: bands, sampleKeys: samples };
  }, [envelope]);

  // Build CSV export input for the current view.
  const csvInput: CsvExportInput | null = useMemo(() => {
    if (chartData.length === 0 || series.length === 0) return null;
    return {
      screen: "Trends (S-04)",
      methodologyVersion: envelope?.metadata?.methodology_version,
      weightsVersion: envelope?.metadata?.weights_version,
      period,
      variable,
      attribution: envelope?.attribution?.map((a) => ({ provider: a.provider, url: a.url })),
      columns: ["period_start", ...series.map((s) => s.provider)],
      rows: chartData.map((row) => [row.period_start, ...series.map((s) => row[s.provider] ?? null)]),
    };
  }, [chartData, series, envelope, period, variable]);

  if (isLoading) {
    return (
      <section aria-labelledby="trends-heading" aria-busy="true">
        <h1 id="trends-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Trends</h1>
        <SkeletonBlock variant="chart" />
      </section>
    );
  }

  if (error) {
    return (
      <section aria-labelledby="trends-heading">
        <h1 id="trends-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Trends</h1>
        <ErrorPanel message="Unable to load trend data." requestId={error.requestId} onRetry={() => window.location.reload()} />
      </section>
    );
  }

  const freshness = envelope?.freshness as Freshness | undefined;
  const warnings = envelope?.warnings as Warning[] | undefined;
  const attribution = envelope?.attribution as Attribution[] | undefined;
  const methodology = envelope?.metadata?.methodology_version;

  if (chartData.length === 0) {
    return (
      <section aria-labelledby="trends-heading">
        <h1 id="trends-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Trends</h1>
        <EmptyState variant="no-data" title="No trend data" description="Trend data appears after sufficient collection history for the selected location." />
      </section>
    );
  }

  return (
    <section aria-labelledby="trends-heading">
      <h1 id="trends-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Trends</h1>

      {/* Control bar */}
      <div style={{ display: "flex", flexWrap: "wrap", gap: "var(--space-sm)", alignItems: "center", marginBottom: "var(--space-md)" }}>
        <VariableSelector selected={variable} onChange={(v) => setParam("variable", v)} />
        <AggregationSelector selected={aggregation} onChange={(a) => setParam("aggregation", a)} />
        <DateRangePicker selected={period} onChange={(p) => setParam("period", p)} />
        <span style={{ flex: 1 }} />
        <ExportButton exportInput={csvInput} filename={`forecastiq-trends-${variable}-${period}`} />
      </div>

      {freshness?.state === "stale" && freshness.last_updated && (
        <div style={{ marginBottom: "var(--space-md)" }}><StaleBanner lastUpdated={freshness.last_updated} /></div>
      )}
      {warnings && warnings.length > 0 && (
        <div style={{ marginBottom: "var(--space-md)" }}><PartialWarnings warnings={warnings} /></div>
      )}

      <TrendChart data={chartData} series={series} unit={unit} ciBands={ciBands} sampleKeys={sampleKeys} />

      {attribution && (
        <div style={{ marginTop: "var(--space-xl)" }}><AttributionFooter providers={attribution} methodologyVersion={methodology} /></div>
      )}
    </section>
  );
}

export default function TrendsPage() {
  return (
    <Suspense fallback={<SkeletonBlock variant="chart" />}>
      <TrendsContent />
    </Suspense>
  );
}
