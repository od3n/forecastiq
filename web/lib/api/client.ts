import type { Envelope, Problem } from "./types";

// Base URL for the ForecastIQ API. In production the static dashboard calls the
// API origin (Caddy-proxied); locally it defaults to a relative /api/v1.
// Configured via the browser-safe NEXT_PUBLIC_API_BASE_URL.
const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? "/api/v1";

/**
 * ApiError carries the RFC 7807 problem and the correlation request id so the
 * error boundary / panels can surface it (S-15). The message never leaks
 * internals — it is the problem title/detail the server chose to expose.
 */
export class ApiError extends Error {
  readonly status: number;
  readonly problem?: Problem;
  readonly requestId?: string;

  constructor(status: number, problem?: Problem, requestId?: string) {
    super(problem?.detail ?? problem?.title ?? `Request failed (${status})`);
    this.name = "ApiError";
    this.status = status;
    this.problem = problem;
    this.requestId = requestId ?? problem?.request_id;
  }
}

export interface ApiGetOptions {
  signal?: AbortSignal;
  /** Bearer JWT for gated endpoints (public reads omit it). */
  token?: string;
  headers?: Record<string, string>;
}

/**
 * apiGet issues a GET and unwraps the standard success envelope, or throws an
 * ApiError carrying the problem + X-Request-Id. Partial results (HTTP 200 with
 * `warnings[]`) return normally — callers inspect `envelope.warnings`.
 */
export async function apiGet<T>(path: string, opts: ApiGetOptions = {}): Promise<Envelope<T>> {
  const headers: Record<string, string> = { Accept: "application/json", ...opts.headers };
  if (opts.token) headers["Authorization"] = `Bearer ${opts.token}`;

  const res = await fetch(`${API_BASE}${path}`, { headers, signal: opts.signal });
  const requestId = res.headers.get("X-Request-Id") ?? undefined;
  const body: unknown = await res.json().catch(() => undefined);

  if (!res.ok) {
    throw new ApiError(res.status, body as Problem | undefined, requestId);
  }
  return body as Envelope<T>;
}

/** apiBase exposes the resolved base for callers that build URLs (e.g. download links). */
export const apiBase = API_BASE;
