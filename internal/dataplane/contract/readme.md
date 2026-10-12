# contract: Execution capability interfaces and call facts

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Implement all methods of Adapted or Bypass before passing the host to execution. RequestHost defines shared capabilities. RoutePlan is request-private; commit the actual target and complete history after success.

| Entry point or resource | Usage and result |
| --- | --- |
| RequestHost / Adapted / Bypass | Interfaces through which executors request host dependencies |
| RoutePlan / RouteTarget / RouteStep | Request plan, stable target, and fallback path |
| CallNote / SpendLog | Accounting and log fact contracts |

### Integration demo

Save the code as `demo/main.go` inside the repository and run `go run ./demo` from the repository root. Remove the demo directory after use.

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/dataplane/contract"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	plan := contract.RoutePlan{Alias: "demo-model", Target: contract.RouteTarget{DeploymentID: "demo-deployment", Model: "demo-model", Pool: "demo-model"}}
	if plan.Err != nil {
		panic(plan.Err)
	}
	fmt.Println(plan.Alias, plan.Target.DeploymentID, plan.Required)
}
```

Prints demo-model demo-deployment false. This illustrates value types without authorizing a candidate or claiming execution success.

### Errors, state, and side effects

Required targets are not optional preferences and cached targets confer no permissions. The interface package does not connect services, forward HTTP, or enforce business contracts by itself.

## Responsibility

Execution capability interfaces and call facts. No HTTP forwarding, route selection or database writes.

## Review order

host.go: RequestHost → Adapted / Bypass → SpendLog, RoutePlan, CallNote

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

execution consumes; gateway implements; auth, config, router, prefs and provider supply values.

## Verification

Adjacent tests: Interface-only package: composition compilation and backend regression.

From repository root: go test ./internal/dataplane/contract -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
