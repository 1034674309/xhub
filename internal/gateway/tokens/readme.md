# tokens: Local token estimates, parameter discovery and transformation records

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Responsibility

Local token estimates, parameter discovery and transformation records. No vendor inference; estimates are not billing measurements; store failures must be observable.

## Review order

tokens.go: Host / Store → Module → tokenCounter / supportedOpenAIParams / transformRequest

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

Host injects auth/model snapshots; Store supplies KV; llm/estimate counts.

## Verification

Adjacent tests: tokens_test.go

From repository root: go test ./internal/gateway/tokens -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
