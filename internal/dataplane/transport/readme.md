# transport: Native HTTP, payloads, paths, authentication and responses

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Match a registered Hit, validate the deployment binding, parse the body, and build the fixed action URL before sending with the injected HTTP client. ForwardOfficial returns facts; WriteThrough emits the response.

| Entry point or resource | Usage and result |
| --- | --- |
| ParseBypassBody(raw, contentType) | Parse JSON/multipart while retaining vendor fields |
| BypassDeploymentURL(base, hit) | Build a registered action URL with encoded path parameters |
| ForwardOfficial / WriteThrough | Send and forward status, headers, and body |

### Integration demo

Save the code as `demo/main.go` inside the repository and run `go run ./demo` from the repository root. Remove the demo directory after use.

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/dataplane/transport"
	"github.com/sunqirui1987/xhub/internal/provider/contract"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	hit := contract.Hit{Action: contract.Action{UpstreamPath: "/v1/videos/{id}"}, Names: map[string]string{"id": "demo-task"}}
	fmt.Println(transport.BypassDeploymentURL("https://example.com/v1", hit))
}
```

Prints https://example.com/v1/videos/demo-task without contacting example.com.

### Errors, state, and side effects

Only registered methods and paths are allowed. Never use arbitrary client URLs as proxy targets. Replace client credentials with deployment credentials and avoid adding implicit SDK retries.

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
