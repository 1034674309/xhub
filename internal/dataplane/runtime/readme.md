# runtime: Runtime snapshots, cooldown, latency and spend flushing

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Responsibility

Runtime snapshots, cooldown, latency and spend flushing. No caller-protocol parsing; acknowledge flushes only after successful persistence.

## Review order

live.go: Runtime → State → RecordFailure / RecordLatency → Flush / FlushLoop

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

consumer-owned Runtime injects models, concurrency, Redis and spend persistence; gateway/wire.go implements it.

## Verification

Adjacent tests: live_test.go

From repository root: go test ./internal/dataplane/runtime -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
