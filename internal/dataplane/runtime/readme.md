# runtime: Runtime snapshots, cooldown, latency and spend flushing

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Implement Runtime with shared model, busy-count, Redis, and accounting dependencies. Take a State snapshot per request, record execution metrics, and acknowledge queues in Flush only after database commit.

| Entry point or resource | Usage and result |
| --- | --- |
| State(host) | Return a router.State snapshot |
| RecordFailure / RecordLatency / RecordUsage | Update cooldown, latency, and usage hot state |
| Flush / FlushLoop | Flush the usage queue once or periodically |

### Integration demo

This is a Go integration helper for a host. It compiles as a separate package inside the repository; callers must supply the real dependencies described below before execution.

```go
package demo

import (
	"github.com/sunqirui1987/xhub/internal/dataplane/runtime"
	"github.com/sunqirui1987/xhub/internal/router"
)

// RoutingSnapshot 演示每请求运行快照；传入完整宿主，返回调度事实，不发送推理。
func RoutingSnapshot(h runtime.Runtime) router.State {
	return runtime.State(h)
}
```

This helper compiles with a complete host. See gateway/wire.go for production wiring; nil is not a substitute for dependencies.

### Errors, state, and side effects

Without Redis, busy counts are process-local. FlushLoop has no context cancellation parameter; start it once with the process lifecycle. Do not acknowledge failed flushes.

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
