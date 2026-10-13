# Model catalog, route classification, and usage pricing

[简体中文](readme_cn.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Normalize usage, price it at the request start time with CostFromRates or CostAt, and persist the Charge snapshot only after checking priced. IAM owns accounting persistence.

| Entry point or resource | Usage and result |
| --- | --- |
| NormalizeUsage(map) | Normalize reported quantities and media dimensions |
| CostFromRates / CostAt | Return Charge and a priced flag |
| Snapshot / SnapshotUsage | Serialize the pricing facts for the call |

### Integration demo

Save the code as `demo/main.go` inside the repository and run `go run ./demo` from the repository root. Remove the demo directory after use.

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/catalog"
	"time"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	usage := catalog.NormalizeUsage(map[string]any{"prompt_tokens": 100, "completion_tokens": 20})
	rates := []catalog.Rate{
		{Measure: "token", Side: "input", Window: "all", USD: 0.000001},
		{Measure: "token", Side: "output", Window: "all", USD: 0.000002},
	}
	charge, priced := catalog.CostFromRates(rates, usage, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	fmt.Printf("%t %.6f\n", priced, charge.Total)
}
```

Prints true 0.000140 using fixed rates, without depending on a market feed.

### Errors, state, and side effects

Rate.USD is already per base unit; do not divide by UnitSize again. priced=false means unpriced, not free. Catalog refreshes must not rewrite historical bills.

## Overview

`internal/catalog` packages the route catalog, model/provider metadata, credential-field descriptions, market price conversion, runtime price layers, usage normalization, and charge calculation used by the gateway. The process starts with embedded resources, accepts provider-package contributions, can replace the generated market layer from a remote feed, and lets management handlers apply runtime model/provider overrides. The catalog is held in memory under a read/write lock; embedded files are startup inputs rather than files watched at runtime.

Pricing supports token, picture, second, and query measures; input/output sides, cache variants, media variants, and Beijing peak/off-peak windows can select different rates. `Rate.USD` is the price for one base unit after import conversion, so billing multiplies it by measured quantity and never divides by `UnitSize` again. A present zero is a real free rate, while a missing rate leaves that quantity unpriced. Charge snapshots retain both measured usage and the exact applied rates so later reloads do not alter historical bills.

The package also classifies paths for public, inference, data-plane, and removed mixed-route handling. `routes.json` describes route shapes and public response metadata; it is not proof that every listed supplier has a working adapter or account. Credential metadata similarly describes configuration fields only.

## External interfaces

### Route and public-data APIs

`Load() []Route` decodes embedded `routes.json`; malformed JSON logs and yields no routes. `AuthOf`, `IsMixedPath`, `IsPublicPath`, `IsDataPlanePath`, and `IsLLMPrefix` classify a normalized path. `HasPrefix` observes segment boundaries, and `PathMatch` supports one-segment `{name}` plus rest-of-path `:path`/`{name:path}` placeholders. `PublicBody(path)` returns fixed embedded payloads for supported public paths and empty lists for generic catalog entries. These functions are read-only and do not authenticate a request themselves.

`Raw`, `CostMap`, `Count`, `LoadedAt`, `ProviderModels`, and `KnownProvider` expose the live in-memory model catalog and provider index to model listing, wildcard expansion, public hub, and administration handlers. `TokenRates(model)` returns per-base-token input/output rates and `ok=false` when no suitable row exists. `Cost(model, prompt, completion)` is the simple token-only calculator; `Format` emits a compact decimal. Returned maps must be treated as catalog data, while mutation must use the write APIs.

`Providers()` returns provider rows from the price document. `CredentialProviders()` merges those rows with the separately embedded credential registry and provider contributions, supplies default API bases, applies the DeepSeek API-base input fields, deduplicates by provider/slug, sorts by display name, and deep-copies appended price providers so caller mutation cannot corrupt shared state. It describes UI credential inputs and does not verify adapter support or credentials.

### Pricing and usage APIs

`NormalizeUsage(map[string]any) Usage` reads provider-specific usage shapes into token, cached-token, image, video-second, and query quantities. OpenAI/Responses nested cached tokens are subsets of total prompt/input and are subtracted from uncached input; Anthropic top-level cache read/write values are quantities beside `input_tokens` and are added without double counting. Explicit zero wins over aliases, invalid or contradictory quantities do not become billable usage.

`CostAt(model, usage, startedAt)` selects a live catalog row and returns `(Charge, true)` only when at least one reported quantity has a compatible rate. Unknown models and unpriced measurements return `ok=false`, which means unpriced rather than free. `CostFromRates` prices caller-supplied tables; `RatesFromFlat` and `CostFromFlatOrRates` convert flat fields while preserving missing-versus-zero semantics. `DecodeRates` accepts the embedded or deployment JSON shapes. `WindowAt`/`IsPeakHour` choose the window from call start time. `Snapshot` stores applied rates, and `SnapshotUsage` stores measured async usage, rates, and whether pricing succeeded.

### Catalog mutation and feeds

`Contribute(Row)` lets provider packages add a missing model and official-ID alias without replacing embedded pricing; `ContributeProvider(ProviderRow)` adds credential/dropdown metadata. Empty IDs/providers are ignored. `BaselineModel`, `ModelRow`, `IsBaseline`, `SetModel`, and `RemoveModel` distinguish generated baseline from live model rows. Provider equivalents are `SetProvider`, `RemoveProvider`, and `IsBaselineProvider`. Set/remove calls rebuild affected indexes under the global lock; they mutate process memory and callers are responsible for durable override storage.

`BuildPriceDocument(raw)` validates and converts a market response into `PriceDocument`; it rejects malformed input and an empty model set. `FetchMarket(ctx, url)` performs the HTTP fetch with context and returns transport/status/decode errors. `ReloadFromMarket` fetches then calls `ApplyDocument`; `ApplyDocument` atomically replaces the generated live layer, rejects an empty document, rebuilds provider indexes, and reapplies local provider/model contributions and overrides. A failed fetch or conversion leaves the active catalog unchanged. `PriceSource`, `PriceGeneratedAt`, `MarkReloaded`, and `EnvForced` report provenance/reload state and the local-only environment switch.

### Actual HTTP registration and callers

This package mounts no handlers. `internal/gateway/models/mount.go` uses it for `GET /v1/models`, `GET /models`, routing-group/default/fallback/available/info endpoints, `GET /model/cost_map`, source/reload/schedule endpoints, `GET /price/catalog`, model/provider price writes and resets, and model CRUD/enable/disable/builtin operations. `internal/gateway/routes.go` registers `GET /public/v1/model_hub`, its facet route, `/public/model_hub`, `/public/model_hub/info`, `/public/endpoints`, `/model_hub`, its facet route, and `POST /model_hub/update_useful_links`. Inference and usage handlers call normalization and pricing before IAM persists a snapshot; provider packages call contribution APIs during registration. HTTP validation, authorization, persistent overrides, scheduling, and status mapping remain in gateway/store layers.

## Core implementation

### Layered catalog and concurrency

Startup embeds and parses `pricedata.json`, indexes every model by `litellm_provider`, and records load time. The implementation keeps immutable baseline model/provider maps apart from mutable live maps. Market documents replace the generated layer, while operator rows and provider-package contributions are restored on top; explicit seller/source binding prevents an unrelated feed source from stealing a priced row. All reads and writes use `modelCostMu`; writes rebuild aliases and provider indexes before releasing it, so readers observe a complete old or new catalog. Empty documents are refused to prevent a bad feed from wiping billing data.

Model lookup builds ordered candidate keys from the requested gateway name, provider-prefixed forms, official aliases, and price-model bindings. It skips non-object or unpriced shells until it finds a compatible row. `KnownProvider` controls when a leading segment is a supplier prefix during wildcard expansion, avoiding accidental stripping of organization/model names. Contributions remember official aliases and price source, but do not overwrite a row already supplied by the baseline.

### Feed conversion and rate protocol

`feed.go` accepts the market response's alternate field shapes, normalizes provider and model identity, mode, variants, unit size, and raw price fields, then emits a `PriceDocument`. Known feed keys map to explicit `measure`, `side`, `variant`, and `window`; unknown keys go through documented string inference and emit debug diagnostics. `measureOf` maps token, second, picture, and query units, while `perUnit` converts quoted unit bundles to dollars per base unit exactly once. Models with metadata but no pricing remain discoverable and unpriced; genuine numeric zero is retained.

A `Rate` carries measure, side, optional variant/window, USD, source field, and original unit metadata. Rate selection first matches measured dimension and side, then the concrete usage variant and call-time window. An unqualified rate can cover all windows; a missing peak rate is not silently replaced with a cheaper off-peak rate when the table explicitly distinguishes windows. Cache reads never fall back to ordinary input pricing. Contradictory media modality details cause pricing failure rather than selecting an arbitrary band. Applied rates and quantities are accumulated separately in `Charge`.

### Usage normalization and time windows

Private numeric readers track presence independently from value, preserving null/missing/explicit-zero distinctions across aliases. Cache normalization clamps nested cached subsets to total input; top-level parallel cache quantities remain additive. Image modalities, resolutions, seconds, and search queries populate separate measure/variant buckets so one measure cannot bleed into another. Zero total measured quantity is not considered a priced call.

Peak time is Monday-Friday 09:00–12:00 and 14:00–18:00 in `Asia/Shanghai`, with half-open boundaries. Embedded Chinese holiday data forces public holidays off and designated make-up weekends on. If timezone data is unavailable, initialization falls back to fixed UTC+8; unreadable holiday JSON logs an error and degrades to weekday-only windows. The call start time alone selects the price window.

### Credential metadata, failures, and verification

`provider_fields.json` is parsed separately from price providers. Credential merging copies nested maps/slices, applies known API-base defaults, appends contributed fields only when no provider/slug collision exists, and adds DeepSeek's `api_base` input field. The accompanying `provider_fields.LICENSE` records the upstream license; distributors must preserve it. Parsing failures are logged and produce the remaining valid sources rather than aborting model pricing.

Tests cover route authorization regressions, credential-provider merging and isolation, feed conversion and empty-feed rejection, rate/window/cache/media/query selection, explicit zero and usage aliases, seller binding, and Chinese holiday/make-up-day boundaries. Remote fetch behavior requires network access only when explicitly exercised; adapter availability, supplier credentials, persistent operator overrides, and IAM accounting are integration boundaries outside this package.

## Source responsibilities and entry points

- [`classify.go`](classify.go) defines `Route`/`AuthClass`, parses embedded routes, classifies public/data-plane/LLM/mixed paths, matches path templates, and constructs fixed public bodies.
- [`contribute.go`](contribute.go) defines model/provider contribution records, protected contribution/official-alias registries, `Contribute`, and `ContributeProvider`; private helpers convert contributed rows and fields into catalog JSON.
- [`cost.go`](cost.go) provides the token-only `Cost` entry point and decimal `Format`, both backed by current catalog token rates.
- [`cost_at.go`](cost_at.go) defines normalized `Usage`, `Charge`, `AppliedRate`, and `PriceSnapshot`; it decodes rates, converts flat fields, normalizes provider usage, resolves candidate model keys, selects variants/windows, computes charges, and serializes snapshots.
- [`credential_providers.go`](credential_providers.go) parses `provider_fields.json`, merges credential and price-provider metadata, deep-copies additions, deduplicates, fills API-base/DeepSeek defaults, and returns sorted `CredentialProviders`.
- [`embed.go`](embed.go) embeds `routes.json`, autorouter presets, price data, and credential fields; `Embedded` validates JSON and centralizes startup logging.
- [`feed.go`](feed.go) defines market wire structures and custom decoding, converts raw price keys and units into `PriceDocument`, performs context-aware HTTP fetches, rejects empty documents, and exposes `BuildPriceDocument`, `FetchMarket`, and `ReloadFromMarket`.
- [`holiday.go`](holiday.go) embeds/parses Chinese holiday exceptions, initializes the Shanghai timezone/fallback, and implements `IsPeakHour`.
- [`model_cost.go`](model_cost.go) initializes the catalog, owns the global lock/live map/provider index/load timestamp/known-provider set, and exports raw/map/count/token/provider/provenance read APIs.
- [`price_write.go`](price_write.go) maintains baseline and live model/provider layers, deep-copy helpers, override/contribution replay, atomic document application, and model/provider set/remove/lookups.
- [`pricedata.go`](pricedata.go) defines `PriceDocument`, decodes the embedded baseline and provenance, exposes provider metadata, and builds baseline price/provider state.
- [`rates.go`](rates.go) defines the `Rate` protocol and private feed-field classification, measure/side/variant/window inference, numeric extraction, and quoted-unit to base-unit conversion.
- [`routes.json`](routes.json) is the embedded route/public-response catalog consumed by `Load`, classification helpers, and `PublicBody`; it describes protocol paths rather than runtime adapter readiness.
- [`publicdata`](publicdata/readme.md) contains embedded prices, Chinese calendar exceptions, credential fields and their license, and autorouter presets; its README documents each resource format and consumer.
