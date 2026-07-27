"use client";

import useSWR, { type SWRConfiguration } from "swr";
import { apiGet, ApiError, type ApiGetOptions } from "./client";
import type { Envelope } from "./types";

/**
 * In local dev (Supabase not configured), automatically inject the dev bearer
 * token so authenticated endpoints (e.g. /me, admin routes) work without a real
 * session. The token value comes from NEXT_PUBLIC_DEV_TOKEN (.env.local).
 * Gated in code to development builds (DRB-WP23L-004): a production bundle
 * never carries the token even if the env var is set in CI by mistake.
 */
const DEV_TOKEN: string | undefined =
  process.env.NODE_ENV === "development" ? process.env.NEXT_PUBLIC_DEV_TOKEN : undefined;

/**
 * devAuthHeaders returns JSON + dev-auth headers for hand-rolled admin
 * mutations (single source for the dev-token injection rule above).
 */
export function devAuthHeaders(): Record<string, string> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (DEV_TOKEN) headers["Authorization"] = `Bearer ${DEV_TOKEN}`;
  return headers;
}

export interface UseApiOptions extends SWRConfiguration {
  /** Bearer JWT for gated endpoints (public reads omit it). */
  token?: string;
  /** Pass `null` as path to conditionally skip the request. */
  skip?: boolean;
}

/**
 * useApi is the data-fetching hook for the ForecastIQ API. It wraps SWR with
 * `apiGet` as the fetcher and returns the standard envelope (or ApiError on
 * failure). Partial results (HTTP 200 + warnings[]) are normal success; the
 * caller inspects `data.warnings`.
 *
 * Pass `skip: true` (or path `null`) to conditionally disable the fetch
 * (e.g. when location_id is not yet resolved).
 */
export function useApi<T>(path: string | null, opts: UseApiOptions = {}) {
  const { token, skip, ...swrOpts } = opts;
  const key = skip || path === null ? null : path;

  const fetcherOpts: ApiGetOptions = {};
  if (token) {
    fetcherOpts.token = token;
  } else if (DEV_TOKEN) {
    fetcherOpts.token = DEV_TOKEN;
  }

  const { data, error, isLoading, isValidating, mutate } = useSWR<Envelope<T>, ApiError>(
    key,
    (p: string) => apiGet<T>(p, fetcherOpts),
    { revalidateOnFocus: false, ...swrOpts },
  );

  return { data, error, isLoading, isValidating, mutate };
}
