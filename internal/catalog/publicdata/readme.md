# Embedded public catalog resources

[简体中文](readme_cn.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

This directory contains build-time resources, not a Go API. Preserve JSON schemas, provenance, and licenses when editing. Rebuild/restart to update embedded baselines; runtime overrides use catalog APIs.

| Entry point or resource | Usage and result |
| --- | --- |
| pricedata.json | Embedded price/supplier baseline |
| cn_holidays.json | Beijing-time pricing calendar |
| provider_fields.json | Supplier credential-field descriptions |

### Integration demo

Run the following commands from the repository root.

```sh
python3 - <<'PY'
import json
from pathlib import Path
for name in ("pricedata.json", "cn_holidays.json", "provider_fields.json"):
    json.loads((Path("internal/catalog/publicdata") / name).read_text())
    print(name, "valid JSON")
PY
go test ./internal/catalog -count=1
```

Run from the repository root to check all three JSON files, then test the consuming package. This does not update deployed resources.

### Errors, state, and side effects

Valid JSON does not prove rate, holiday, or credential-field correctness; check consumer semantics as well. Editing files does not hot-update running processes.

## Overview

`internal/catalog/publicdata` contains build-time data consumed by the parent `catalog` package. It has no Go package, mutable state, or HTTP handler. The resources provide the baseline price/provider catalog, the Beijing billing calendar, a provider credential-field snapshot and its license, plus a retained autorouter preset document. `go:embed` puts their bytes into the binary; editing files cannot change a running process. Runtime market reloads and operator overrides are separate layers.

## External interfaces

The directory exports no Go symbols. Parent-package consumers are the interface:

- `pricedata.json` is decoded at catalog initialization into `PriceDocument`; `CostAt`, `CostMap`, `Providers`, model/provider mutation, and gateway model/price endpoints consume the resulting in-memory catalog.
- `cn_holidays.json` is parsed by `holiday.go` for `IsPeakHour`, affecting peak/off-peak billing snapshots.
- `provider_fields.json` is parsed by `CredentialProviders`, then returned through catalog public bodies used by model-hub/provider-form endpoints.
- `autorouter_presets.json` is an embedded resource; request routing does not read it.

Actual HTTP consumers are registered above this package: public model catalog routes include `GET /public/v1/model_hub`, `/public/v1/model_hub/{facet}`, `/public/model_hub`, `/public/model_hub/info`, `/public/endpoints`, `/model_hub`, and `/model_hub/{facet}`. Administrative price and provider mutations live under `/price/catalog`, `/price/model`, and `/price/provider`. Parsing or shape errors surface through parent initialization/reload behavior; this directory itself cannot return errors.

## Core implementation

### Resource contracts

`pricedata.json` is a JSON document with source metadata, generation time, provider rows, and model rows. Model IDs are stable lookup keys. Rate rows preserve measure, side, variant, window, unit size, source key, and per-base-unit USD; explicit zero is a real price and must not be treated as missing. Parent code copies the embedded baseline before applying provider contributions, feed replacement, and operator overrides.

`cn_holidays.json` separates statutory holidays from weekend make-up workdays. `holiday.go` evaluates Asia/Shanghai weekdays and half-open 09:00–12:00 and 14:00–18:00 windows, then applies these exceptions. Calendar updates affect future calculations after rebuild but never rewrite stored price snapshots.

`provider_fields.json` is a JSON array of provider descriptors and credential fields such as key, label, field type, required flag, defaults, and placeholders. `CredentialProviders` merges it with local provider rows, fills known default API bases, performs DeepSeek normalization, deduplicates provider/slug, deep-copies appended rows, and sorts by display name. These forms describe configuration; they do not prove that an adapter or external account is available.

`provider_fields.LICENSE` records the license/provenance obligations for the credential-field snapshot and must travel with updates to that derived data. `autorouter_presets.json` is not read by request routing. All JSON must remain valid because malformed embedded resources can leave catalogs empty or cause fallback/error behavior during process startup.

### Verification

There are no direct tests in this resource-only directory. `internal/catalog` tests verify feed decoding, credential-provider merging and isolation, holidays/window boundaries, rate selection, explicit-zero pricing, aliases, and image/video variants. Run `go test ./internal/catalog -count=1`; rebuilding is required to validate changed embedded bytes. External provider availability and market-source freshness remain outside these fixtures.

## Source responsibilities and entry points

### `pricedata.json`

Generated baseline price catalog: metadata, provider dropdown rows, model properties, aliases, flat rate fields, and normalized rate tables. Embedded and decoded by `pricedata.go`/`model_cost.go`; billed by `cost_at.go`; exposed and mutated through gateway model handlers.

### `cn_holidays.json`

Chinese holiday and adjusted-workday calendar consumed by `holiday.go` to classify Beijing peak windows. It is calendar data, not a scheduler.

### `provider_fields.json`

Credential-form snapshot consumed by `credential_providers.go`. It supplies provider display metadata and field schemas before local providers/default API bases are merged.

### `provider_fields.LICENSE`

License notice for `provider_fields.json`. It has no runtime parser but is a required companion resource for redistribution and updates.

### `autorouter_presets.json`

Retained preset JSON embedded by the catalog resource bundle. No current route-selection entry point is enabled merely by this file.
