# diagnostics: Raw upstream facts and safe diagnostic logs

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Responsibility

Raw upstream facts and safe diagnostic logs. No pricing or generated-content storage; routing logs must not leak credentials.

## Review order

upstream_diagnostics.go → log.go → host.go: TraceHop

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

execution and transport collect facts; gateway/requestlog redacts and serializes.

## Verification

Adjacent tests: upstream_diagnostics_test.go

From repository root: go test ./internal/dataplane/diagnostics -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
