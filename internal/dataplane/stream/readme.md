# stream: Raw stream forwarding and read-only usage observation

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Responsibility

Raw stream forwarding and read-only usage observation. Do not re-encode native events; capture limits must not truncate forwarded bytes; return read failures.

## Review order

stream.go: PipeStream → streamUsageParser → ResponseID

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

execution calls; usage normalizes statistics; dialogue owns protocol-changing SSE.

## Verification

Adjacent tests: responses_continuation_test.go, usage_stream_test.go

From repository root: go test ./internal/dataplane/stream -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
