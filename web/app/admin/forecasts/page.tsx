"use client";

import { useState, useMemo } from "react";
import { useApi } from "@/lib/api/hooks";
import { absoluteLocal } from "@/lib/format";
import { SkeletonBlock } from "@/components/SkeletonBlock";
import { ErrorPanel } from "@/components/ErrorPanel";

interface ProviderRef { id: string; name: string; }
interface LocationRef { id: string; name: string; timezone?: string; }

interface Snapshot {
  id: string;
  target_time: string;
  issued_at: string;
  forecast_horizon_minutes: number;
  temperature_c: number | null;
  feels_like_temperature_c: number | null;
  precipitation_probability: number | null;
  precipitation_amount_mm: number | null;
  humidity_pct: number | null;
  wind_speed_ms: number | null;
  wind_direction_deg: number | null;
  pressure_hpa: number | null;
  cloud_cover_pct: number | null;
  canonical_condition_code: string | null;
}

interface LatestForecastData {
  collection: {
    id: string;
    status: string;
    completed_at: string;
    records_received: number;
    snapshots_stored: number;
    schema_version: string;
  };
  snapshots: Snapshot[];
}

interface CollectionRow {
  id: string;
  status: string;
  requested_at: string;
  snapshots_stored: number;
}

interface CollectionsList {
  collections: CollectionRow[];
}

const cell: React.CSSProperties = { padding: "var(--space-xs) var(--space-sm)", fontFamily: "var(--font-data)", fontSize: "var(--text-body-sm)", whiteSpace: "nowrap" };
const head: React.CSSProperties = { padding: "var(--space-sm)", textAlign: "left", whiteSpace: "nowrap" };

// Admin raw forecast viewer: the latest collection's snapshots for a selected
// provider × location (GET /forecasts/latest; read:data-gated). Read-only.
export default function AdminForecastsPage() {
  const { data: providersEnv } = useApi<{ providers: ProviderRef[] }>("/providers");
  const { data: locationsEnv } = useApi<{ locations: LocationRef[] }>("/locations");

  const providers = providersEnv?.data?.providers ?? [];
  const locations = locationsEnv?.data?.locations ?? [];

  const [providerId, setProviderId] = useState("");
  const [locationId, setLocationId] = useState("");
  const [dayFilter, setDayFilter] = useState("all");
  const [collectionId, setCollectionId] = useState("latest");

  // Default to the first provider/location once loaded.
  const effProvider = providerId || providers[0]?.id || "";
  const effLocation = locationId || locations[0]?.id || "";

  // Recent collection runs for the historical picker (admin lineage API).
  const collectionsPath = effProvider && effLocation
    ? `/forecast-collections?provider_id=${effProvider}&location_id=${effLocation}&limit=20`
    : null;
  const { data: collectionsEnv } = useApi<CollectionsList>(collectionsPath);
  const recentCollections = collectionsEnv?.data?.collections ?? [];

  // "latest" → the public latest-forecast endpoint; a specific id → the
  // admin snapshots-by-collection endpoint (same response shape).
  const path = effProvider && effLocation
    ? (collectionId === "latest"
        ? `/forecasts/latest?provider_id=${effProvider}&location_id=${effLocation}`
        : `/forecast-collections/${collectionId}/snapshots`)
    : null;
  const { data: envelope, error, isLoading, mutate } = useApi<LatestForecastData>(path);

  const snapshots = useMemo(() => envelope?.data?.snapshots ?? [], [envelope]);
  const collection = envelope?.data?.collection;

  // Distinct days present in the snapshot set (local dates), for the filter.
  const days = useMemo(() => {
    const set = new Set<string>();
    for (const s of snapshots) {
      const d = new Date(s.target_time);
      set.add(`${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`);
    }
    return Array.from(set).sort();
  }, [snapshots]);

  const todayKey = useMemo(() => {
    const d = new Date();
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
  }, []);

  const filtered = useMemo(() => {
    if (dayFilter === "all") return snapshots;
    return snapshots.filter((s) => {
      const d = new Date(s.target_time);
      const key = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
      return key === dayFilter;
    });
  }, [snapshots, dayFilter]);

  function dayLabel(key: string): string {
    if (key === todayKey) return `Today (${key})`;
    return key < todayKey ? `${key} (past)` : key;
  }

  return (
    <section aria-labelledby="forecasts-heading">
      <h1 id="forecasts-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Raw Forecasts</h1>

      {/* Selectors */}
      <div style={{ display: "flex", gap: "var(--space-sm)", alignItems: "center", marginBottom: "var(--space-md)", flexWrap: "wrap" }}>
        <label style={{ fontSize: "var(--text-body-sm)", color: "var(--color-text-secondary)" }}>
          Provider{" "}
          <select value={effProvider} onChange={(e) => { setProviderId(e.target.value); setCollectionId("latest"); setDayFilter("all"); }} style={{ padding: "var(--space-xs)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-sm)", fontFamily: "inherit" }}>
            {providers.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
          </select>
        </label>
        <label style={{ fontSize: "var(--text-body-sm)", color: "var(--color-text-secondary)" }}>
          Location{" "}
          <select value={effLocation} onChange={(e) => { setLocationId(e.target.value); setCollectionId("latest"); setDayFilter("all"); }} style={{ padding: "var(--space-xs)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-sm)", fontFamily: "inherit" }}>
            {locations.map((l) => <option key={l.id} value={l.id}>{l.name}</option>)}
          </select>
        </label>
        <label style={{ fontSize: "var(--text-body-sm)", color: "var(--color-text-secondary)" }}>
          Collection{" "}
          <select value={collectionId} onChange={(e) => { setCollectionId(e.target.value); setDayFilter("all"); }} style={{ padding: "var(--space-xs)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-sm)", fontFamily: "inherit" }}>
            <option value="latest">Latest successful</option>
            {recentCollections.map((cl) => (
              <option key={cl.id} value={cl.id}>
                {absoluteLocal(cl.requested_at)} — {cl.status} ({cl.snapshots_stored})
              </option>
            ))}
          </select>
        </label>
        <label style={{ fontSize: "var(--text-body-sm)", color: "var(--color-text-secondary)" }}>
          Day{" "}
          <select value={dayFilter} onChange={(e) => setDayFilter(e.target.value)} style={{ padding: "var(--space-xs)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-sm)", fontFamily: "inherit" }}>
            <option value="all">All days ({snapshots.length} rows)</option>
            {days.map((d) => <option key={d} value={d}>{dayLabel(d)}</option>)}
          </select>
        </label>
      </div>

      {/* Collection lineage summary */}
      {collection && (
        <p style={{ color: "var(--color-text-secondary)", fontSize: "var(--text-body-sm)", marginBottom: "var(--space-md)" }}>
          Latest collection <span style={{ fontFamily: "var(--font-data)" }}>{collection.id.slice(0, 8)}…</span>{" "}
          — status <strong>{collection.status}</strong>, {collection.records_received} received / {collection.snapshots_stored} stored,{" "}
          completed {absoluteLocal(collection.completed_at)} ({collection.schema_version})
        </p>
      )}

      {isLoading && <SkeletonBlock variant="row" count={8} />}
      {error && <ErrorPanel message="Unable to load raw forecast data." requestId={error.requestId} onRetry={() => mutate()} />}

      {!isLoading && !error && filtered.length === 0 && (
        <p style={{ color: "var(--color-text-secondary)" }}>No snapshots for this selection.</p>
      )}

      {!isLoading && !error && filtered.length > 0 && (
        <div className="tableWrap" style={{ overflowX: "auto" }}>
          <table style={{ width: "100%", borderCollapse: "collapse", minWidth: 900 }}>
            <thead>
              <tr style={{ background: "var(--color-surface-secondary)", fontSize: "var(--text-label)", textTransform: "uppercase" }}>
                <th scope="col" style={head}>Target Time</th>
                <th scope="col" style={head}>Horizon</th>
                <th scope="col" style={head}>Temp (°C)</th>
                <th scope="col" style={head}>Feels (°C)</th>
                <th scope="col" style={head}>Rain Prob</th>
                <th scope="col" style={head}>Rain (mm)</th>
                <th scope="col" style={head}>Humidity</th>
                <th scope="col" style={head}>Wind (m/s)</th>
                <th scope="col" style={head}>Dir (°)</th>
                <th scope="col" style={head}>Pressure</th>
                <th scope="col" style={head}>Cloud</th>
                <th scope="col" style={head}>Condition</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((s) => (
                <tr key={s.id} style={{ borderBottom: "1px solid var(--color-border)" }}>
                  <td style={cell}>{absoluteLocal(s.target_time)}</td>
                  <td style={cell}>+{s.forecast_horizon_minutes / 60}h</td>
                  <td style={cell}>{s.temperature_c ?? "—"}</td>
                  <td style={cell}>{s.feels_like_temperature_c ?? "—"}</td>
                  <td style={cell}>{s.precipitation_probability !== null ? `${Math.round(s.precipitation_probability * 100)}%` : "—"}</td>
                  <td style={cell}>{s.precipitation_amount_mm ?? "—"}</td>
                  <td style={cell}>{s.humidity_pct !== null ? `${s.humidity_pct}%` : "—"}</td>
                  <td style={cell}>{s.wind_speed_ms ?? "—"}</td>
                  <td style={cell}>{s.wind_direction_deg ?? "—"}</td>
                  <td style={cell}>{s.pressure_hpa ?? "—"}</td>
                  <td style={cell}>{s.cloud_cover_pct !== null ? `${s.cloud_cover_pct}%` : "—"}</td>
                  <td style={cell}>{s.canonical_condition_code ?? "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
