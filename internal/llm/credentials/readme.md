# credentials

[简体中文](readme_cn.md)

Trusted credential hydration.

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Read named credentials from trusted storage, hydrate current deployment parameters, then validate the transport and connection. Use a fresh snapshot rather than writing it back to the input map.

| Entry point or resource | Usage and result |
| --- | --- |
| Hydrate(params, credentialValues) | Return a new top-level map with environment expansion |

### Integration demo

Save the code as `demo/main.go` inside the repository and run `go run ./demo` from the repository root. Remove the demo directory after use.

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/llm/credentials"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	params := map[string]any{"model": "demo-model", "api_key": "old-demo-key"}
	values := map[string]any{"api_key": "new-demo-key", "custom_llm_provider": "OpenAI"}
	hydrated := credentials.Hydrate(params, values)
	fmt.Println(hydrated["custom_llm_provider"], params["api_key"] == "old-demo-key")
}
```

Prints openai true. The new credential replaces the output key while the input remains unchanged. The example does not print credentials.

### Errors, state, and side effects

Only api_key/api_base/custom_llm_provider are copied from credentials; non-key fields only fill blanks. The copy is shallow, so nested values remain shared. No automatic AWS/Azure authentication is provided.

## Source and verification

Named credentials supply only the fields consumed by the execution layer: `api_key`, `api_base`, and `custom_llm_provider`. A nonblank credential key replaces the deployment key; other fields fill missing values. Environment references are resolved at hydration time. Existing deployment parameters are preserved.

Azure, AWS, and Vertex automatic authentication fields are not imported from named credentials because this layer has no corresponding authentication executor. Guardrail AWS configuration belongs to its own module.

Code entry points: [credential.go](credential.go)

Unit tests live beside source: [hydrate_test.go](hydrate_test.go)

go test ./internal/llm/credentials/...

Backend regressions live in cmd/regression; browser flows in frontend/e2e. Offline tests use local HTTP upstreams and do not establish production credential or vendor availability.
