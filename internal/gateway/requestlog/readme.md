# requestlog: Content encoding, streamed-log reconstruction and secret redaction

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Encode a completed exchange into three log strings and persist them through authorized log storage. Serialize upstream diagnostics separately with DiagnosticJSON. Reconstructed logs are not client responses.

| Entry point or resource | Usage and result |
| --- | --- |
| Encode(r, requestBytes, responseBytes) | Return messages/response/proxy JSON strings |
| DiagnosticJSON(facts, usage) | Return redacted diagnostic text |

### Integration demo

Save the code as `demo/main.go` inside the repository and run `go run ./demo` from the repository root. Remove the demo directory after use.

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/gateway/requestlog"
	"net/http/httptest"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	r := httptest.NewRequest("POST", "/v1/responses", nil)
	r.Header.Set("Authorization", "Bearer demo-key")
	messages, response, proxy := requestlog.Encode(r, []byte(`{"input":"Hello"}`), []byte(`{"id":"demo-response","output":[]}`))
	fmt.Println(messages != "", response != "", proxy != "")
}
```

Prints true true true. Returned strings are log fields; the example does not display their contents.

### Errors, state, and side effects

Redaction does not replace log-read authorization. Higher-level settings govern prompt retention. SSE log reconstruction must not re-encode the client stream.

## Responsibility

Content encoding, streamed-log reconstruction and secret redaction. No pricing or upstream calls; log reconstruction must not change responses; raw and billing usage remain comparable.

## Review order

encode.go: Encode → assembleLoggedResponse / redactHeaders → diagnostics.go: DiagnosticJSON

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

inputs are values/facts; gateway/spend.go owns lifecycle/persistence.

## Verification

Adjacent tests: encode_test.go, prompt_response_test.go

From repository root: go test ./internal/gateway/requestlog -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
