"use client";

import { useCallback } from "react";
import { useApi } from "@/lib/api/hooks";
import { LocationAdminTable, type LocationEntry } from "@/components/LocationAdminTable";
import { SkeletonBlock } from "@/components/SkeletonBlock";
import { ErrorPanel } from "@/components/ErrorPanel";

interface LocationsData { locations: LocationEntry[]; }

export default function AdminLocationsPage() {
  const { data: envelope, error, isLoading, mutate } = useApi<LocationsData>("/locations");

  const handleToggle = useCallback(async (id: string, newStatus: string) => {
    await fetch(`/api/v1/locations/${id}/status`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ status: newStatus }) });
    mutate();
  }, [mutate]);

  const handleCreate = useCallback(async (data: { name: string; country_code: string; latitude: number; longitude: number; timezone: string }): Promise<string | null> => {
    const resp = await fetch("/api/v1/locations", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(data) });
    if (resp.status === 409) return "A location already exists within 50 km (BR-LOC-01).";
    if (!resp.ok) return "Failed to create location.";
    mutate();
    return null;
  }, [mutate]);

  if (isLoading) return (<section><h1 style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Locations</h1><SkeletonBlock variant="row" count={4} /></section>);
  if (error) return (<section><h1 style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Locations</h1><ErrorPanel message="Unable to load locations." requestId={error.requestId} onRetry={() => mutate()} /></section>);

  const locations = envelope?.data?.locations ?? [];

  return (
    <section>
      <h1 style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Locations</h1>
      {locations.length === 0 ? <p style={{ color: "var(--color-text-secondary)" }}>No locations configured.</p> : null}
      <LocationAdminTable locations={locations} onToggleStatus={handleToggle} onCreate={handleCreate} />
    </section>
  );
}
