# models: Deployments, credentials, capabilities and routing configuration

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Responsibility

Deployments, credentials, capabilities and routing configuration. No per-request inference; review validation before writes, discovery and deletion cleanup.

## Review order

host.go / mount.go → admin.go / validate.go → available.go / discovery.go → groups.go / fallback.go / routing_groups.go → price.go / schedule.go

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

config/provider declare capabilities; catalog supplies prices; Host injects storage/auth/probes.

## Verification

Adjacent tests: admin_test.go, available_test.go, builtin_test.go, discovery_test.go, fallback_test.go, groups_test.go, list_test.go, price_selection_test.go, price_test.go, routing_groups_test.go, schedule_test.go, validate_credential_test.go

From repository root: go test ./internal/gateway/models -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
