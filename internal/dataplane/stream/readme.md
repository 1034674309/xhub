# stream: Raw stream forwarding and read-only usage observation

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Forward native SSE once with PipeStream and use its completion, usage, first-byte duration, and captured log output. Cross-protocol event conversion belongs to dataplane/dialogue.

| Entry point or resource | Usage and result |
| --- | --- |
| PipeStream(w, resp, start) | Write raw client bytes and return observed results/errors |
| ResponseID(raw) | Read an ID from response JSON or complete SSE |

### Integration demo

Save the code as `demo/main.go` inside the repository and run `go run ./demo` from the repository root. Remove the demo directory after use.

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/dataplane/stream"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	fmt.Println(stream.ResponseID([]byte(`{"id":"demo-response","object":"response"}`)))
}
```

Prints demo-response in an offline ID extraction example. Package tests exercise PipeStream byte preservation and read errors.

### Errors, state, and side effects

Log capture limits must not truncate client output. Propagate read errors. Once output starts, never append another vendor response or publish failed-stream continuation history.

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
