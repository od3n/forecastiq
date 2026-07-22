# ForecastIQ — Provider Adapter Authoring Guide

**Version**: 1.0
**Status**: Approved — WP-05 (Provider Adapter Framework)
**Authority**: `docs/planning/05-implementation-work-packages.md` §WP-05; `docs/workflows/01-forecast-collection.md`; `docs/operations/06-provider-failure-runbook.md` (FC-08..FC-13)

This guide explains how to add a new forecast provider behind the hardened
adapter framework. The framework is provider-agnostic: transport hardening,
retry, rate-limit normalization, and failure classification are shared, so a
new adapter only implements provider-specific request shaping, schema
validation, normalization, and condition mapping.

Reference implementation: `adapters/forecastproviders/openmeteo`.

---

## 1. The port

Implement `internal/collection/ports.ForecastProviderAdapter`:

```go
type ForecastProviderAdapter interface {
	Slug() string            // stable provider slug (matches providers.slug)
	SchemaVersion() string   // e.g. "openweather-v1"; bump on breaking schema change
	AdapterVersion() string  // semver; bump on adapter logic change
	Capabilities() Capabilities
	FetchForecast(ctx, ForecastRequest) (*ForecastResult, error)
}
```

`Slug`, `SchemaVersion`, and `AdapterVersion` are recorded on **every**
collection row (workflow §9) — they are the lineage that lets replay and drift
diagnosis reason about which code produced which data. Rules:

- **Slug** is immutable and must equal the seeded `providers.slug`.
- **SchemaVersion** identifies the response contract you parse. Bump it when the
  provider's payload shape changes in a way that required an adapter change
  (runbook §3, schema drift).
- **AdapterVersion** is semver for your adapter's own logic; bump the minor when
  behaviour changes without a schema change.

### Return contract

`FetchForecast` returns a `*ForecastResult` and a **nil** Go error for all
*classified* outcomes (success, partial, and every provider/config failure).
Reserve a non-nil Go error only for programmer errors that should abort the
pipeline. The service inspects `result.Outcome` and `result.ErrorCode`, not the
Go error, for classified failures. See `openmeteo.FetchForecast`.

Populate at minimum: `RawPayload`, `Checksum` (`ports.Checksum(raw)`),
`HTTPStatusCode`, `LatencyMS`, `SchemaVersion`, `AdapterVersion`, `IssuedAt`,
`Outcome`, and — on failure — `ErrorCode`.

## 2. Use the shared transport (`providerhttp`)

Do **not** hand-roll HTTP, retry, or rate-limit parsing. Build a
`providerhttp.Client` and call `Get`:

```go
transport := providerhttp.New(providerhttp.Config{
	HTTPClient:       cfg.Client,       // optional; a hardened default is used otherwise
	Limiter:          cfg.Limiter,      // optional in-process token bucket (ADR-020)
	MaxResponseBytes: cfg.MaxRespBytes, // defaults to 10 MB
	MaxAttempts:      cfg.MaxRetries,   // FC-08 caps at 5
	RetryBaseDelay:   time.Second,      // 1,2,4,8,16s ±20% jitter
})

resp, ferr := transport.Get(ctx, endpoint, header)
```

The transport gives you, for free:

- a stable **User-Agent** and a **capped redirect** policy;
- **bounded** response-body reads (fails closed on oversized payloads);
- **FC-08 retry/backoff** (retryable = network / timeout / 5xx / 429);
- **FC-13 classification** into a `*ports.ProviderError` (`ferr` above);
- **rate-limit normalization** from `Retry-After` / `X-RateLimit-*` headers into
  `resp.RateLimit` / `ferr.RateLimit`.

`resp` is always non-nil and carries best-effort `Body`/`StatusCode`/`LatencyMS`
even on failure, so you can still persist the raw error payload (ADR-011).

### Mapping a failure onto the result

```go
if ferr != nil {
	result.Outcome = ferr.Outcome()      // timeout / rate_limited / auth_failed / failed
	result.ErrorCode = ferr.Code.String()// canonical FC-13 code
	result.Err = ferr
	result.RateLimit = ferr.RateLimit
	return result, nil
}
```

## 3. Error taxonomy (FC-13)

Classify **only** into the canonical `ports.ErrorCode` set
(runbook §1): `timeout`, `provider_5xx`, `rate_limited`, `schema_drift`,
`network_local`, `db_error`, `payload_write_failed`, `circuit_open`,
`invalid_credentials`. The transport assigns the transport/HTTP codes; your
decode step assigns `schema_drift` (see below). Never invent new codes — the
reliability/coverage accounting depends on the closed set.

## 4. Schema validation & normalization

Own the provider-specific decode in a `decompose`-style function:

- Reject the payload as `schema_drift` when a required field is missing or when
  **> 50 %** of rows are invalid; classify as `OutcomePartial` when some rows
  are invalid; otherwise `OutcomeSuccess` (see `openmeteo/decompose.go`).
- Normalize to canonical units at the source: **UTC** timestamps, precipitation
  probability as a **[0,1]** ratio, etc. (domain §5).
- Map provider condition codes to the canonical taxonomy and tally unmapped
  codes into `result.UnmappedConditions` (FC-15) — never guess a mapping.
- Precompute snapshot IDs with `ids.New()` (UUIDv7, ADR-022). Do **not** set
  `ForecastCollectionID` — the service sets it before persistence.

## 5. Capabilities

Declare what you support so the composition root and operators can reason about
the provider without provider-specific knowledge:

```go
func (a *Adapter) Capabilities() ports.Capabilities {
	return ports.Capabilities{
		MaxForecastHorizon: 7 * 24 * time.Hour,
		HourlyResolution:   true,
		RequiresCredential: false,
		SupportsReplay:     true,
	}
}
```

If you set `SupportsReplay: true` you **must** also implement
`ports.ReplayDecoder` (`DecodeStored`) — the registry rejects the adapter at
startup otherwise.

## 6. Replay (`ReplayDecoder`)

Implement deterministic decode from stored bytes with **no** network call
(domain §4.8). Reuse the same decode path as `FetchForecast`:

```go
func (a *Adapter) DecodeStored(_ context.Context, req ports.ForecastRequest, raw []byte) (*ports.ForecastResult, error)
```

Determinism is a contract: the same payload + request must yield the same
checksum and snapshot set on every run. Cover it with a replay test.

## 7. Registration

Register the adapter in the composition root
(`cmd/forecastiq/app.go`), never in application/domain code:

```go
registry := collection.NewRegistry()
if err := registry.Register(myAdapter); err != nil { return nil, err }
```

The registry validates identity/versions, rejects duplicate slugs, and enforces
the replay-capability contract. Registered providers are logged at startup
(`provider.registered`) for operational inspection.

## 8. Security

- Base URLs are **seeded provider configuration**, never user input — do not
  fetch user-supplied URLs (no SSRF surface; security architecture §5).
- **Never** log credentials, `Authorization` headers, raw response bodies, or
  query strings. `ports.ProviderError.Error()` is already redacted (code +
  status only); keep your own logs at the same standard.
- Resolve credentials from `req.Credential` (the service resolves the
  `credential_ref` from env); never read env directly in the adapter.

## 9. Testing (contract matrix)

Add deterministic, offline contract tests using committed fixtures and
`httptest` servers (see `openmeteo/openmeteo_test.go` and
`docs/testing/03-contract-testing.md` §1.2). Cover at least:

- success, edge/null values, partial-invalid, schema-drift;
- rate-limited (assert `RateLimit` metadata), auth-failed, server-error;
- unmapped condition tally;
- replay determinism (`DecodeStored`).

**Never call the real provider API in CI.** Live smoke tests are manual and
documented separately (WP-06 for Open-Meteo).

## 10. Checklist

- [ ] Slug matches the seeded provider; schema/adapter versions set.
- [ ] Uses `providerhttp.Client`; no hand-rolled retry/HTTP.
- [ ] Classifies only into canonical FC-13 codes.
- [ ] Normalizes to UTC + canonical units; maps conditions; tallies unmapped.
- [ ] Declares `Capabilities`; implements `ReplayDecoder` if replay is declared.
- [ ] Registered in `cmd/forecastiq/app.go`.
- [ ] No secrets/bodies/URLs logged.
- [ ] Offline contract-test matrix green (incl. replay), with `-race`.
