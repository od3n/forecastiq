"use client";

import { useSearchParams, useRouter, usePathname } from "next/navigation";
import { useCallback, useState, useEffect } from "react";

/** The default horizon (+24h = 1440 minutes) per doc 02 §14.2. */
export const DEFAULT_HORIZON = 1440;

const HORIZON_STORAGE_KEY = "fiq_horizon_minutes";

function storeHorizon(minutes: number): void {
  if (typeof window === "undefined") return;
  localStorage.setItem(HORIZON_STORAGE_KEY, String(minutes));
}

/** Horizon options (segmented control values per doc 02 §3.1). */
export const HORIZON_OPTIONS = [
  { label: "+1h", minutes: 60 },
  { label: "+3h", minutes: 180 },
  { label: "+6h", minutes: 360 },
  { label: "+12h", minutes: 720 },
  { label: "+24h", minutes: 1440 },
  { label: "+3d", minutes: 4320 },
  { label: "+7d", minutes: 10080 },
] as const;

export interface GlobalParams {
  locationId: string | null;
  horizonMinutes: number;
}

/**
 * useGlobalParams reads + writes the URL-synced global controls (location_id +
 * horizon_minutes). These persist across navigation (shareable URLs; doc 02
 * §14.2). horizon_minutes also persists to localStorage so the user's last
 * selection is restored on fresh page loads. Defaults: first active location
 * (resolved by the LocationSelector) and +24h.
 */
export function useGlobalParams() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();

  // Read localStorage after mount to avoid SSR/hydration mismatch.
  const [storedHorizon, setStoredHorizon] = useState<number | null>(null);
  useEffect(() => {
    const stored = localStorage.getItem(HORIZON_STORAGE_KEY);
    if (stored) {
      const n = Number(stored);
      if (Number.isFinite(n) && n > 0) setStoredHorizon(n);
    }
  }, []);

  const locationId = searchParams.get("location_id");
  const horizonFromUrl = searchParams.get("horizon_minutes");
  const horizonMinutes = horizonFromUrl
    ? Number(horizonFromUrl)
    : (storedHorizon ?? DEFAULT_HORIZON);

  const setParams = useCallback(
    (updates: Partial<Record<"location_id" | "horizon_minutes", string>>) => {
      const params = new URLSearchParams(searchParams.toString());
      for (const [k, v] of Object.entries(updates)) {
        if (v) params.set(k, v);
        else params.delete(k);
      }
      // Persist horizon selection to localStorage.
      if (updates.horizon_minutes) {
        storeHorizon(Number(updates.horizon_minutes));
      }
      router.replace(`${pathname}?${params.toString()}`, { scroll: false });
    },
    [searchParams, router, pathname],
  );

  return { locationId, horizonMinutes, setParams } as const;
}
