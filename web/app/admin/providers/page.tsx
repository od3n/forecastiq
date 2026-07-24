"use client";

import { useCallback } from "react";
import { useApi } from "@/lib/api/hooks";
import { ProviderAdminTable, type ProviderEntry } from "@/components/ProviderAdminTable";
import { SkeletonBlock } from "@/components/SkeletonBlock";
import { ErrorPanel } from "@/components/ErrorPanel";

interface ProvidersData {
  providers: ProviderEntry[];
}

// S-11 Admin Providers (doc 02 §4.11). Enable/disable + config edit (minute
// offset). Credential status shown as "Configured"/"Not set" (never exposes
// secrets per BR-08). The admin layout handles the role guard.
export default function AdminProvidersPage() {
  const { data: envelope, error, isLoading, mutate } = useApi<ProvidersData>("/providers");

  const handleToggle = useCallback(async (id: string, newStatus: string) => {
    await fetch(`/api/v1/admin/providers/${id}/status`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ status: newStatus }),
    });
    mutate();
  }, [mutate]);

  const handleEditConfig = useCallback(async (id: string, minuteOffset: number) => {
    await fetch(`/api/v1/admin/provider-configurations/${id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ minute_offset: minuteOffset }),
    });
    mutate();
  }, [mutate]);

  if (isLoading) {
    return (
      <section aria-labelledby="providers-heading">
        <h1 id="providers-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Providers</h1>
        <SkeletonBlock variant="row" count={4} />
      </section>
    );
  }

  if (error) {
    return (
      <section aria-labelledby="providers-heading">
        <h1 id="providers-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Providers</h1>
        <ErrorPanel message="Unable to load providers." requestId={error.requestId} onRetry={() => mutate()} />
      </section>
    );
  }

  const providers = envelope?.data?.providers ?? [];

  return (
    <section aria-labelledby="providers-heading">
      <h1 id="providers-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Providers</h1>
      {providers.length === 0 ? (
        <p style={{ color: "var(--color-text-secondary)" }}>No providers configured.</p>
      ) : (
        <ProviderAdminTable providers={providers} onToggleStatus={handleToggle} onEditConfig={handleEditConfig} />
      )}
    </section>
  );
}
