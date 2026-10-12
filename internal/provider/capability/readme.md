# capability

[简体中文](readme_cn.md)

Public endpoints and protocol matching.

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Use EndpointTypes for management choices and EndpointForOp/CompatibleEndpoint for runtime validation. Endpoint IDs describe public entry points; transport IDs describe upstream execution and cannot be inferred from one another.

| Entry point or resource | Usage and result |
| --- | --- |
| EndpointTypes / KnownEndpoint | Public endpoint catalog and existence check |
| CompatibleEndpoint(endpoint, transport) | Check endpoint/transport compatibility |
| DialogueProtocol(protocol) | Check unified-dialogue codec support |

### Integration demo

Save the code as `demo/main.go` inside the repository and run `go run ./demo` from the repository root. Remove the demo directory after use.

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/provider/capability"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	fmt.Println(capability.KnownEndpoint("responses"))
	fmt.Println(capability.DialogueProtocol("anthropic-messages"))
	fmt.Println(capability.KnownEndpoint("unknown-demo"))
}
```

Prints true, true, and false. Catalog presence does not mean every deployment can execute the endpoint.

### Errors, state, and side effects

Do not infer capabilities from URLs, supplier names, or token prices. New protocols require endpoint/execution registration and compatibility/failure tests.

## Source and verification

Code entry points: [capability.go](capability.go), [endpoints.go](endpoints.go), [execution.go](execution.go)

Unit tests live beside source: [capability_test.go](capability_test.go)

go test ./internal/provider/capability/...

Backend regressions live in cmd/regression; browser flows in frontend/e2e. Offline tests use local HTTP upstreams and do not establish production credential or vendor availability.
