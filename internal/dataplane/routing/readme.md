# routing: Candidate filtering, model fallbacks and routing diagnostics

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Responsibility

Candidate filtering, model fallbacks and routing diagnostics. No upstream calls or settlement; distinguish missing, disabled, cooling, unallocated and incompatible candidates.

## Review order

serve.go: DropDisabled → fallback.go → route_diagnostic.go: DiagnoseRoute

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

provider validates capabilities; router supplies policy/state; execution and previews share diagnostics.

## Verification

Adjacent tests: disabled_test.go, fallback_boundary_test.go, fallback_test.go, prefer_test.go, route_diagnostic_test.go

From repository root: go test ./internal/dataplane/routing -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
