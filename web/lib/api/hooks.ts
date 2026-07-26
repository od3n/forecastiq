"use client";

import useSWR, { type SWRConfiguration } from "swr";
import { apiGet, ApiError, type ApiGetOptions } from "./client";
import { getAccessToken } from "@/lib/auth/session";
import type { Envelope } from "./types";

export interface UseApiOptions extends SWRConfiguration {
  /** Explicit bearer token override; when omitted the session token is used. */
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

  // The bearer is resolved per request inside the fetcher: the explicit token
  // override wins, otherwise the current session token (Supabase session or
  // dev-mode token; see lib/auth/session). Signed-out requests go bare —
  // public reads succeed, gated endpoints 401 and the guards redirect.
  const fetcher = async (p: string) => {
    const fetcherOpts: ApiGetOptions = {};
    const bearer = token ?? (await getAccessToken());
    if (bearer) fetcherOpts.token = bearer;
    return apiGet<T>(p, fetcherOpts);
  };

  const { data, error, isLoading, isValidating, mutate } = useSWR<Envelope<T>, ApiError>(
    key,
    fetcher,
    { revalidateOnFocus: false, ...swrOpts },
  );

  return { data, error, isLoading, isValidating, mutate };
}
