// Standard response-envelope types (docs/api/02-response-conventions.md §1-4).
// The generated OpenAPI types (./generated.ts) are the contract-drift artifact;
// these hand-written types give the envelope ergonomic generics the spec's
// generic `object` schemas cannot.

/** The four server-computed freshness states (BR-FRESH-02; never client-derived). */
export type FreshnessState = "fresh" | "delayed" | "stale" | "unavailable";

export interface Freshness {
  state: FreshnessState;
  last_updated?: string;
  age_seconds?: number;
  threshold_seconds?: number;
  /** Present only when state = "unavailable". */
  reason?: string;
}

/** Per-provider partial-result warning (top-level, never nested per row). */
export interface Warning {
  provider_id?: string;
  code: "provider_unavailable" | "stale" | (string & {});
  message: string;
  since?: string;
}

/** Provider attribution, configured server-side (BR-ATTR-01; never hardcoded). */
export interface Attribution {
  provider: string;
  text: string;
  url: string;
}

export interface Pagination {
  next_cursor?: string;
  has_more: boolean;
}

export interface Metadata {
  request_id: string;
  generated_at?: string;
  timezone?: string;
  units?: Record<string, string>;
  methodology_version?: string;
  weights_version?: string;
}

/** The standard success envelope wrapping every 2xx payload. */
export interface Envelope<T> {
  data: T;
  metadata: Metadata;
  freshness?: Freshness;
  provenance?: Record<string, unknown>;
  attribution?: Attribution[];
  partial_result?: boolean;
  warnings?: Warning[];
  pagination?: Pagination;
}

/** RFC 7807 problem + ForecastIQ extensions (error contracts §2). */
export interface Problem {
  type: string;
  title: string;
  status: number;
  detail?: string;
  instance?: string;
  request_id?: string;
  retryable?: boolean;
  docs?: string;
  errors?: { field: string; message: string }[];
}
