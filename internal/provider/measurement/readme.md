# measurement

[简体中文](readme_cn.md)

Response/SSE usage and task billing facts.

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Inject NativeUsage or task Billing callbacks during transport registration. Pass raw responses and trusted creation context to them, then price the results with catalog. These callbacks do not calculate money or persist bills.

| Entry point or resource | Usage and result |
| --- | --- |
| NativeUsage(protocol) | Return a response/request usage extractor |
| NativeStreamUsage(protocol, raw, request) | Extract usage from complete SSE events |
| SeedanceBilling / FalBilling | Build task context and successful-terminal extractors |

### Integration demo

Save the code as `demo/main.go` inside the repository and run `go run ./demo` from the repository root. Remove the demo directory after use.

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/provider/measurement"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	extract := measurement.NativeUsage("openai-chat")
	usage := extract(map[string]any{"usage": map[string]any{"prompt_tokens": 100, "completion_tokens": 20}}, nil)
	fmt.Println(usage["prompt_tokens"], usage["completion_tokens"])
}
```

Prints 100 20 using an offline extraction fixture.

### Errors, state, and side effects

Preserve pricing_blocked when usage or media dimensions are missing. Do not assume free usage or equate request quantities with output. Failed/incomplete tasks must not be charged from requested quantities.

## Source and verification

Code entry points: [fal_billing.go](fal_billing.go), [native_usage.go](native_usage.go), [seedance_billing.go](seedance_billing.go)

Unit tests live beside source: [bindings_test.go](bindings_test.go), [google_usage_test.go](google_usage_test.go), [native_usage_test.go](native_usage_test.go)

go test ./internal/provider/measurement/...

Backend regressions live in cmd/regression; browser flows in frontend/e2e. Offline tests use local HTTP upstreams and do not establish production credential or vendor availability.
