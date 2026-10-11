# guard: Guard configuration, rules and external runners

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Responsibility

Guard configuration, rules and external runners. No model routing or pricing; review external failures, redaction and blocking together.

## Review order

host.go / mount.go → manage.go → guard.go / runner.go → custom_http.go / external.go

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

Host/store, builtins and external runners compose; gateway hooks enforce outcomes.

## Verification

Adjacent tests: custom_test.go, engine_test.go, external_test.go, guard_regression_test.go, guard_test.go, primitives_test.go

From repository root: go test ./internal/gateway/guard -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
