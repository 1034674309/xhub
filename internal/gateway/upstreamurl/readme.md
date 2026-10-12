# upstreamurl

[简体中文](readme_cn.md)

Build explicit upstream URLs for ingress contract display. Tests cover prefixes, trailing slashes, invalid URLs and traversal boundaries.

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Pass a trusted deployment api_base, fixed endpoint, and provider ID to PassthroughURL when constructing compatible upstream URLs. Transport still owns authentication and sending.

| Entry point or resource | Usage and result |
| --- | --- |
| PassthroughURL(base, endpoint, provider) | Preserve the base path while joining the fixed endpoint |

### Integration demo

Save the code as `demo/main.go` inside the repository and run `go run ./demo` from the repository root. Remove the demo directory after use.

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/gateway/upstreamurl"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	fmt.Println(upstreamurl.PassthroughURL("https://api.openai.com", "/chat/completions", "openai"))
}
```

Prints https://api.openai.com/v1/chat/completions without a network request.

### Errors, state, and side effects

Only the official OpenAI host gets /v1 supplementation. Preserve relay base paths. Invalid parsing may return the base unchanged; callers must validate deployment URLs.

## Source and verification

go test ./internal/gateway/upstreamurl
