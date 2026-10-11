# execution: Adapted and native request lifecycle orchestration

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Responsibility

Adapted and native request lifecycle orchestration. No console CRUD or direct PostgreSQL/Redis access; review retries, failed billing and successful completion together.

## Review order

serve.go: Serve → official.go: ServeBypass

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

contract injects identity, quotas, credentials, HTTP and spend; routing, dialogue, transport, stream, usage and diagnostics handle specialized work.

## Verification

Adjacent tests: bypass_logic_test.go, cache_scope_test.go, capability_test.go, endpoint_redesign_test.go, failure_log_test.go, fal_test.go, fallback_boundary_test.go, fallback_test.go, native_bypass_test.go, official_settlement_test.go, official_template_test.go, qiniu_live_test.go, route_diagnostic_test.go, stream_failure_regression_test.go, task_status_test.go, usage_stream_test.go

From repository root: go test ./internal/dataplane/execution -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
