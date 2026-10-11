# usage: Reports, activity, guard statistics and log reads

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Responsibility

Reports, activity, guard statistics and log reads. No spend creation or vendor calls; review tenant authorization before filtering, pagination and amounts.

## Review order

host.go / mount.go → reports.go → activity.go / entity_activity.go → guardrails.go

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

Host supplies scoped identity/storage; IAM/store query persisted data.

## Verification

Adjacent tests: activity_test.go, cost_breakdown_test.go, guardrails_test.go, health_connection_test.go, log_guardrail_test.go, reports_test.go

From repository root: go test ./internal/gateway/usage -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
