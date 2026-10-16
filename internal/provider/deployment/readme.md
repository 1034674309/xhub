# deployment

[简体中文](readme_cn.md)

Explicit bindings, validation and candidates.

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

ValidateDeployment during startup and management writes, then revalidate current configuration with Execution/AllowsEndpoint/ResolveHit during execution. Specify model_info.transport and endpoint_types explicitly.

| Entry point or resource | Usage and result |
| --- | --- |
| ValidateDeployment(model) | Reject invalid configuration |
| Execution(model) | Return an explicitly bound transport and bool |
| Candidates / DeploymentEndpoints | Filter candidates and describe endpoint bindings |

### Integration demo

Save the code as `demo/main.go` inside the repository and run `go run ./demo` from the repository root. Remove the demo directory after use.

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/config"
	_ "github.com/sunqirui1987/xhub/internal/provider/all"
	"github.com/sunqirui1987/xhub/internal/provider/deployment"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	model := config.ModelEntry{ModelName: "demo-model", LiteLLMParams: map[string]any{"model": "demo-upstream"}, ModelInfo: map[string]any{"transport": "bypass_openai_responses", "endpoint_types": []string{"bypass:openai-responses"}}}
	if err := deployment.ValidateDeployment(model); err != nil {
		panic(err)
	}
	transport, ok := deployment.Execution(model)
	fmt.Println(ok, transport.ID)
}
```

Prints true bypass_openai_responses. It checks protocol binding, not URLs, credentials, or connectivity.

### Errors, state, and side effects

Do not infer execution from api_base/custom_llm_provider when declarations are missing. Capability compatibility still requires disabled-state, authorization, budget, and weight checks.

## Source and verification

Code entry points: [bindings.go](bindings.go), [candidates.go](candidates.go), [catalog_binding.go](catalog_binding.go), [execution.go](execution.go), [registry.go](registry.go), [validate.go](validate.go)

Unit tests live beside source: [bindings_test.go](bindings_test.go), [candidates_test.go](candidates_test.go), [catalog_binding_test.go](catalog_binding_test.go), [endpoint_redesign_test.go](endpoint_redesign_test.go), [registry_test.go](registry_test.go), [validate_test.go](validate_test.go)

go test ./internal/provider/deployment/...

Backend regressions live in cmd/regression; browser flows in frontend/e2e. Offline tests use local HTTP upstreams and do not establish production credential or vendor availability.

### Optional Responses web search

Deployment metadata `web_search_support` accepts `inherit` (default; preserve native
Responses behavior), `supported` (requires native Responses transport), or `unsupported`.
`web_search_fallback` accepts `reject` (default) or explicit `drop_optional`.
`EncodeDialogue` applies the same policy for candidate selection, preview and execution.
Only optional Responses `web_search`/`web_search_preview` declarations are removed,
including Codex `additional_tools`. Required/tool-selected search and other unsupported
capabilities remain rejected. This does not execute searches or infer support from model names.
Responses expose `X-XHub-Dropped-Capabilities: web_search`; request diagnostics persist
`dropped_capabilities`, and the log drawer displays a localized downgrade notice.
