# dialogue: Unified dialogue conversion, continuation and adapted SSE

[中文](readme_cn.md) · [Parent architecture](../readme.md)

`PipeDialogue` preserves and validates native SSE for matching OpenAI protocols; other combinations reuse the shared stream state machine. `ResponseHistory` preserves native reasoning/tool items. Standard endpoints still enforce authorization, budgets and guardrails rather than invoking bypass execution. See the [compatibility matrix](../../../docs/development/en/agent-protocol-compatibility.md).

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

After recovering Responses history, combine it with the incremental input using ResponsesRequestBody before guardrail and budget checks. Build from a hydrated deployment and let the executor send the request.

| Entry point or resource | Usage and result |
| --- | --- |
| ResponsesRequestBody(body, history) | Return a merged request or error |
| BuildDialogue / BuildRegisteredOperation | Build an upstream request using an explicit transport |
| ProbeDialogue(ctx, client, dep) | Send a real minimal probe request |

### Integration demo

Save the code as `demo/main.go` inside the repository and run `go run ./demo` from the repository root. Remove the demo directory after use.

```go
package main

import (
	"fmt"
	dpdialogue "github.com/sunqirui1987/xhub/internal/dataplane/dialogue"
	llmdialogue "github.com/sunqirui1987/xhub/internal/llm/dialogue"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	history := []llmdialogue.Turn{{Role: "user", Text: "Hello"}, {Role: "assistant", Text: "Hi"}}
	body, err := dpdialogue.ResponsesRequestBody(map[string]any{"input": "Continue", "previous_response_id": "demo-response"}, history)
	if err != nil {
		panic(err)
	}
	d, err := llmdialogue.ParseDialogue("openai-responses", body)
	if err != nil {
		panic(err)
	}
	fmt.Println(len(d.Turns), body["previous_response_id"] == nil)
}
```

Prints 3 true: two historical turns plus the current input, with the parent ID consumed by the gateway.

### Errors, state, and side effects

Current input must remain valid. StripGatewayFields mutates its map. ProbeDialogue contacts the upstream and may incur charges; it is not a pure validator.

## Responsibility

Unified dialogue conversion, continuation and adapted SSE. Convert supported text and function tools; fail explicitly on unsupported capabilities, retaining native capabilities through bypass.

## Review order

dialogue.go: BuildDialogue, ResponsesRequestBody, ProbeDialogue → dialogue_stream.go: PipeDialogue

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

llm owns protocol values and official SDK types; provider declares transports; transport handles native forwarding.

## Verification

Adjacent tests: dialogue_probe_test.go, dialogue_stream_test.go, registered_operation_test.go, responses_continuation_test.go, sdk_stream_test.go

From repository root: go test ./internal/dataplane/dialogue -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
