# registry

[简体中文](readme_cn.md)

Built-in registration and path matching.

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Blank-import provider/all at startup and match requests with Match(method,path), then validate the deployment. Registration is a startup concern; requests must not upload arbitrary protocol paths.

| Entry point or resource | Usage and result |
| --- | --- |
| RegisterTransport / RegisterModel / RegisterSupplier | Register built-in protocols and catalog contributions |
| Match(method, path) | Return Hit,bool; unknown paths do not match |
| Transports / PublicBody | Return protocol catalog views |

### Integration demo

Save the code as `demo/main.go` inside the repository and run `go run ./demo` from the repository root. Remove the demo directory after use.

```go
package main

import (
	"fmt"
	_ "github.com/sunqirui1987/xhub/internal/provider/all"
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

Complete global registration during startup. Transports returns copies. A path match does not authorize a request or establish an available deployment. Importing does not connect, provision credentials, or verify supplier availability. The data plane must deduplicate task settlement.

## Source and verification

Code entry points: [catalog_binding.go](catalog_binding.go), [match.go](match.go), [models.go](models.go), [registry.go](registry.go), [suppliers.go](suppliers.go)

Unit tests live beside source: [catalog_binding_test.go](catalog_binding_test.go), [endpoint_redesign_test.go](endpoint_redesign_test.go), [isolation_test.go](isolation_test.go), [match_test.go](match_test.go), [registry_test.go](registry_test.go)

go test ./internal/provider/registry/...

Registration and reads isolate mutable slices, maps and billing callback containers. Match accepts only built-in actions, never deployment-supplied paths.

Backend regressions live in cmd/regression; browser flows in frontend/e2e. Offline tests use local HTTP upstreams and do not establish production credential or vendor availability.

## Coverage gate

Run scripts/core-codec-coverage.sh using bash. Registry and dialogue codec must each reach 95% statement coverage. Test failures, missing evidence, or a lower percentage fail the command. The profile uses package unit tests without adding backend regression or browser coverage.
