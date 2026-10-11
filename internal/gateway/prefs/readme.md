# prefs: Preference/route-setting reads, merges and persistence

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Responsibility

Preference/route-setting reads, merges and persistence. No routing execution; absence differs from zero; merge order determines effective settings.

## Review order

host.go / mount.go → settings.go / page.go → route_settings.go / merge.go

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

Host injects storage/auth/reload; router defines policy values.

## Verification

Adjacent tests: allocation_test.go, fallback_test.go, route_settings_test.go, routing_groups_test.go

From repository root: go test ./internal/gateway/prefs -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
