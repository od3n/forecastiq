"use client";

import useSWR, { type SWRConfiguration } from "swr";
import { apiGet, ApiError, type ApiGetOptions } from "./client";
import type { Envelope } from "./types";

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
  if (token) fetcherOpts.token = token;

  const { data, error, isLoading, isValidating, mutate } = useSWR<Envelope<T>, ApiError>(
    key,
    (p: string) => apiGet<T>(p, fetcherOpts),
    { revalidateOnFocus: false, ...swrOpts },
  );

  return { data, error, isLoading, isValidating, mutate };
}
