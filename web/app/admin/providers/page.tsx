"use client";

import { useCallback, useMemo } from "react";
import { useApi, devAuthHeaders as authHeaders } from "@/lib/api/hooks";
import { apiBase } from "@/lib/api/client";
import { ProviderAdminTable, type ProviderEntry } from "@/components/ProviderAdminTable";
import { SkeletonBlock } from "@/components/SkeletonBlock";
import { ErrorPanel } from "@/components/ErrorPanel";

interface ApiProvider {
  id: string;
  name: string;
  slug: string;
  status: string;
  collecting_since?: string | null;
}

interface ProvidersData { providers: ApiProvider[]; }

interface ApiProviderConfig {
  id: string;
  provider_id: string;
  status: string;
  has_credential: boolean;
  collection_schedule: { interval: string; minute_offset: number };
}

interface ConfigsData { configurations: ApiProviderConfig[]; }

// S-11 Admin Providers (doc 02 §4.11). Enable/disable + config edit (minute
// offset). Credential status shown as "Configured"/"Not set" (never exposes
// secrets per BR-08). The admin layout handles the role guard.
export default function AdminProvidersPage() {
  const { data: envelope, error, isLoading, mutate } = useApi<ProvidersData>("/providers");
  const { data: configsEnv, mutate: mutateConfigs } = useApi<ConfigsData>("/admin/provider-configurations");

  // Merge provider identity with its operational configuration.
  const providers: ProviderEntry[] = useMemo(() => {
    const configByProvider = new Map(
      (configsEnv?.data?.configurations ?? []).map((c) => [c.provider_id, c]),
    );
    return (envelope?.data?.providers ?? []).map((p) => {
      const cfg = configByProvider.get(p.id);
      return {
        id: p.id,
        name: p.name,
        slug: p.slug,
        status: p.status,
        has_credential: cfg?.has_credential ?? false,
        collecting_since: p.collecting_since ?? null,
        minute_offset: cfg?.collection_schedule?.minute_offset ?? 0,
        config_id: cfg?.id,
        config_status: cfg?.status,
      };
    });
  }, [envelope, configsEnv]);

  const handleToggle = useCallback(async (id: string, newStatus: string) => {
    await fetch(`${apiBase}/admin/providers/${id}/status`, {
      method: "PATCH",
      headers: authHeaders(),
      body: JSON.stringify({ status: newStatus }),
    });
    mutate();
    mutateConfigs();
  }, [mutate, mutateConfigs]);

  const handleEditConfig = useCallback(async (providerId: string, minuteOffset: number) => {
    // The PATCH targets the configuration id, not the provider id.
    const cfg = (configsEnv?.data?.configurations ?? []).find((c) => c.provider_id === providerId);
    if (!cfg) return;
    await fetch(`${apiBase}/admin/provider-configurations/${cfg.id}`, {
      method: "PATCH",
      headers: authHeaders(),
      body: JSON.stringify({ minute_offset: minuteOffset }),
    });
    mutate();
    mutateConfigs();
  }, [configsEnv, mutate, mutateConfigs]);

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
