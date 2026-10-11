# contract: Execution capability interfaces and call facts

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Responsibility

Execution capability interfaces and call facts. No HTTP forwarding, route selection or database writes.

## Review order

host.go: RequestHost → Adapted / Bypass → SpendLog, RoutePlan, CallNote

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

execution consumes; gateway implements; auth, config, router, prefs and provider supply values.

## Verification

Adjacent tests: Interface-only package: composition compilation and backend regression.

From repository root: go test ./internal/dataplane/contract -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
