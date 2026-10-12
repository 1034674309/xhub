# contract

[简体中文](readme_cn.md)

Shared value contracts.

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Define Transport/Action/AuthConfig when adding a protocol and register it with registry. The data plane consumes matched Hits. TaskContext stores minimal creation-time pricing context; actual quantities come from successful terminal results.

| Entry point or resource | Usage and result |
| --- | --- |
| Transport / Action / Hit | Protocol, whitelisted actions, and path captures |
| ReadTaskID(doc, path) | Read a task ID from a dotted path |
| OfficialID(prefix, stored) | Strip one supplier prefix |

### Integration demo

Save the code as `demo/main.go` inside the repository and run `go run ./demo` from the repository root. Remove the demo directory after use.

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/provider/contract"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	doc := map[string]any{"data": map[string]any{"task_id": "demo-task"}}
	fmt.Println(contract.ReadTaskID(doc, "data.task_id"))
	fmt.Println(contract.OfficialID("demo", "demo/org/model"))
}
```

Prints demo-task and org/model. These pure functions do not register protocols or send requests.

### Errors, state, and side effects

Expand performs text substitution without URL escaping; use transport builders for outgoing paths. Task context is not measured usage and must not contain credentials or prompts.

## Source and verification

Code entry points: [type.go](type.go)

Unit tests live beside source: [match_test.go](match_test.go)

go test ./internal/provider/contract/...

Backend regressions live in cmd/regression; browser flows in frontend/e2e. Offline tests use local HTTP upstreams and do not establish production credential or vendor availability.
