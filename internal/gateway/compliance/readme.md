# compliance: Compliance-check endpoints for request logs

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Responsibility

Compliance-check endpoints for request logs. Computes submitted-record checks; no guard execution, persistence or legal assessment.

## Review order

compliance.go: Host / Module → writeCompliance → euAIActChecks / gdprChecks

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

minimal Host for management authorization; util mounts and writes responses.

## Verification

Adjacent tests: compliance_test.go

From repository root: go test ./internal/gateway/compliance -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
