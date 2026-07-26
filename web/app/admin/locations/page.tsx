"use client";

import { useCallback } from "react";
import { useApi, devAuthHeaders as authHeaders } from "@/lib/api/hooks";
import { apiBase } from "@/lib/api/client";
import { authHeaders } from "@/lib/auth/session";
import { LocationAdminTable, type LocationEntry, type CreateLocationData, type CreateResult } from "@/components/LocationAdminTable";
import { SkeletonBlock } from "@/components/SkeletonBlock";
import { ErrorPanel } from "@/components/ErrorPanel";

interface LocationsData { locations: LocationEntry[]; }

export default function AdminLocationsPage() {
  const { data: envelope, error, isLoading, mutate } = useApi<LocationsData>("/locations");

  const handleToggle = useCallback(async (id: string, newStatus: string) => {
    await fetch(`${apiBase}/locations/${id}/status`, { method: "PATCH", headers: await authHeaders(), body: JSON.stringify({ status: newStatus }) });
    mutate();
  }, [mutate]);

  const handleCreate = useCallback(async (data: CreateLocationData): Promise<CreateResult | null> => {
    const resp = await fetch(`${apiBase}/locations`, { method: "POST", headers: await authHeaders(), body: JSON.stringify(data) });
    if (resp.status === 409) {
      // Surface the conflicting location from the RFC 7807 problem (BR-LOC-01).
      const problem = await resp.json().catch(() => null);
      const existing = problem?.existing_resource?.name;
      return {
        error: existing
          ? `Too close to existing location “${existing}” — locations must be at least ~5.5 km (0.05°) apart.`
          : "A location already exists within ~5.5 km.",
        conflict: true,
      };
    }
    if (!resp.ok) return { error: "Failed to create location." };
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
