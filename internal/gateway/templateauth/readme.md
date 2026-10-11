# templateauth: Route-template selection authorization

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Responsibility

Route-template selection authorization. No template routing; enforce authorization independently of frontend hiding.

## Review order

selection.go: exported validators → callers

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

authz/IAM and injected lookups constrain selection.

## Verification

Adjacent tests: selection_test.go

From repository root: go test ./internal/gateway/templateauth -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
