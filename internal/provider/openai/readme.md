# openai

[简体中文](readme_cn.md)

OpenAI native registration.

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Blank-import this package at startup to register its fixed actions, then match them through registry. Administrators must still configure deployments with explicit transport/endpoint_types, base URLs, and credentials. The native data plane sends requests.

| Entry point or resource | Usage and result |
| --- | --- |
| 空白导入 openai | Trigger init; there is no runtime constructor |
| registry.Match(method, path) | Obtain transport, action, and path captures |

### Integration demo

Save the code as `demo/main.go` inside the repository and run `go run ./demo` from the repository root. Remove the demo directory after use.

```go
package main

import (
	"fmt"
	_ "github.com/sunqirui1987/xhub/internal/provider/openai"
	"github.com/sunqirui1987/xhub/internal/provider/registry"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	hit, ok := registry.Match("POST", "/bypass/openai/v1/responses")
	if !ok {
		panic("registered action not found")
	}
	fmt.Println(hit.Transport.ID, hit.Action.Name)
}
```

Prints bypass_openai_responses create. This only verifies protocol registration; it does not require an account or submit a task.

### Errors, state, and side effects

Deployment credentials use Authorization: Bearer; video settlement requires measured seconds from a completed task. Importing does not connect, provision credentials, or verify supplier availability. The data plane must deduplicate task settlement.

## Source and verification

Code entry points: [native.go](native.go)

Unit tests live beside source: [native_test.go](native_test.go), [registration_test.go](registration_test.go)

go test ./internal/provider/openai/...

Backend regressions live in cmd/regression; browser flows in frontend/e2e. Offline tests use local HTTP upstreams and do not establish production credential or vendor availability.
