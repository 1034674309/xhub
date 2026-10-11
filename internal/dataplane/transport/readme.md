# transport: Native HTTP, payloads, paths, authentication and responses

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Responsibility

Native HTTP, payloads, paths, authentication and responses. Preserve vendor JSON extensions, multipart, statuses, errors and SSE bytes; no hidden SDK retries.

## Review order

bypass_body.go → bypass_url.go / official.go → bypass_forward.go: ForwardOfficial, WriteThrough

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

injects http.Client and provider.Hit/Transport; diagnostics collects facts; no Server dependency.

## Verification

Adjacent tests: endpoint_redesign_test.go, fallback_test.go, native_bypass_test.go

From repository root: go test ./internal/dataplane/transport -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
