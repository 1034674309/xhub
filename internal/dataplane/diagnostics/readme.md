# diagnostics: Raw upstream facts and safe diagnostic logs

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Install ObserveUpstream before reading the upstream Body and use the collected facts after EOF. TraceFirstByte can instrument the outgoing request. The caller owns closing Body.

| Entry point or resource | Usage and result |
| --- | --- |
| TraceFirstByte(req) | Return an instrumented request and time pointer |
| ObserveUpstream(resp) | Wrap Body and return facts updated during reads |

### Integration demo

Save the code as `demo/main.go` inside the repository and run `go run ./demo` from the repository root. Remove the demo directory after use.

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/dataplane/diagnostics"
	"io"
	"net/http"
	"strings"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"usage":{"prompt_tokens":0}}`))}
	facts := diagnostics.ObserveUpstream(resp)
	defer resp.Body.Close()
	if _, err := io.ReadAll(resp.Body); err != nil {
		panic(err)
	}
	fmt.Println(facts.StatusCode, facts.UsageReported)
}
```

Prints 200 true: an explicit zero is still reported usage. The fixture makes no network calls.

### Errors, state, and side effects

Incomplete reads or parse limits leave incomplete facts. Do not invent successful usage. Logs require redaction; diagnostics do not authorize access or price usage.

## Responsibility

Raw upstream facts and safe diagnostic logs. No pricing or generated-content storage; routing logs must not leak credentials.

## Review order

upstream_diagnostics.go → log.go → host.go: TraceHop

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

execution and transport collect facts; gateway/requestlog redacts and serializes.

## Verification

Adjacent tests: upstream_diagnostics_test.go

From repository root: go test ./internal/dataplane/diagnostics -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
