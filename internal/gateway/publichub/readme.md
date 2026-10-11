# publichub: Public model catalog, filtering, sorting and links

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Responsibility

Public model catalog, filtering, sorting and links. No credential disclosure; handles pagination, case-folded filtering and store failures.

## Review order

public_hub.go: Module → publicModelHub / publicModelHubFacet → filterPublicModels / sortPublicModels

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

catalog supplies public data; minimal Host gates writes; Store persists link KV.

## Verification

Adjacent tests: public_hub_test.go

From repository root: go test ./internal/gateway/publichub -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
