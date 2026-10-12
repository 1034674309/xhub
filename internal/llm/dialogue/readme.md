# dialogue

[简体中文](readme_cn.md)

Unified dialogue codecs and strict validation.

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Select an explicit source protocol, parse and check errors, then encode for the destination protocol and upstream model. Use the corresponding result codecs for responses; dataplane handles network execution.

| Entry point or resource | Usage and result |
| --- | --- |
| ParseDialogue(protocol, body) | Strictly decode into Dialogue |
| EncodeDialogue(dialogue, protocol, model) | Encode a request map for the target protocol |
| ParseDialogueResult / EncodeDialogueResult | Decode and encode response facts |

### Integration demo

Save the code as `demo/main.go` inside the repository and run `go run ./demo` from the repository root. Remove the demo directory after use.

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/llm/dialogue"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	d, err := dialogue.ParseDialogue("openai-responses", map[string]any{"input": "Hello"})
	if err != nil {
		panic(err)
	}
	body, err := dialogue.EncodeDialogue(d, "anthropic-messages", "demo-model")
	if err != nil {
		panic(err)
	}
	fmt.Println(len(d.Turns), body["model"], body["max_tokens"])
}
```

Prints 1 demo-model 4096 with no credentials or network. Protocol IDs are openai-chat, openai-responses, anthropic-messages, gemini, and vertex.

### Errors, state, and side effects

Unsupported fields, roles, and tools must fail rather than silently losing capabilities. Consume gateway control fields upstream; use native forwarding for full vendor extensions.

## Source and verification

Code entry points: [dialogue_google.go](dialogue_google.go), [model.go](model.go), [request.go](request.go), [response.go](response.go), [usage.go](usage.go), [validation.go](validation.go)

Unit tests live beside source: [dialogue_google_test.go](dialogue_google_test.go), [dialogue_sdk_test.go](dialogue_sdk_test.go), [dialogue_test.go](dialogue_test.go)

go test ./internal/llm/dialogue/...

model defines normalized values; request/response encode and decode; validation checks fields; usage normalizes reported facts; dialogue_google handles Google. No network calls.

Backend regressions live in cmd/regression; browser flows in frontend/e2e. Offline tests use local HTTP upstreams and do not establish production credential or vendor availability.

## Coverage gate

Run scripts/core-codec-coverage.sh using bash. Registry and dialogue codec must each reach 95% statement coverage. Test failures, missing evidence, or a lower percentage fail the command. The profile uses package unit tests without adding backend regression or browser coverage.
