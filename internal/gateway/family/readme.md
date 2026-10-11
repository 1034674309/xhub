# family: Catalog resource endpoints and tenant isolation

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Responsibility

Catalog resource endpoints and tenant isolation. Specialized endpoints belong to dedicated modules; review isolation before generic outputs.

## Review order

host.go → mount.go → handlers.go

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

Host supplies identity, scoped storage and catalog metadata; gateway/catalog.go registers routes.

## Verification

Adjacent tests: pagination_test.go, resource_isolation_test.go

From repository root: go test ./internal/gateway/family -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
