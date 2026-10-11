# affinity: Tenant-scoped affinity, response continuation and task ownership

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Responsibility

Tenant-scoped affinity, response continuation and task ownership. No routing policy or pricing; isolate contexts by caller, model and endpoint; never commit failed continuations.

## Review order

affinity.go: New / State → PlanRoute → CommitRoute → PinOfficial / OfficialContext

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

State owns locks, memory and a minimal Redis interface; gateway composes it; execution consumes contract methods.

## Verification

Adjacent tests: affinity_pin_test.go, affinity_test.go, responses_context_test.go

From repository root: go test ./internal/gateway/affinity -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
