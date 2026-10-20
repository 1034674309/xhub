package dialogue

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestToolStopReasonSurvivesConversion 验证完整工具参数与文本共存时，正常和长度截断原因在三协议JSON转换中保持一致。
// 参数t为单测上下文；纯JSON解析/编码，额外拒绝损坏工具参数；无外部依赖或资源清理。
func TestToolStopReasonSurvivesConversion(t *testing.T) {
	for _, source := range []string{"openai-chat", "openai-responses", "anthropic-messages"} {
		for _, limited := range []bool{false, true} {
			stop, status := "tool_calls", "completed"
			if limited {
				stop, status = "length", "incomplete"
			}
			raw := `{"id":"r","choices":[{"message":{"role":"assistant","content":"prefix","tool_calls":[{"id":"c","type":"function","function":{"name":"lookup","arguments":"{}"}}]},"finish_reason":"` + stop + `"}],"usage":{"prompt_tokens":8,"completion_tokens":2}}`
			if source == "openai-responses" {
				raw = `{"id":"r","status":"` + status + `","output":[{"type":"message","content":[{"type":"output_text","text":"prefix"}]},{"type":"function_call","call_id":"c","name":"lookup","arguments":"{}"}],"usage":{"input_tokens":8,"output_tokens":2}}`
			}
			if source == "anthropic-messages" {
				stop = "tool_use"
				if limited {
					stop = "max_tokens"
				}
				raw = `{"id":"r","content":[{"type":"text","text":"prefix"},{"type":"tool_use","id":"c","name":"lookup","input":{}}],"stop_reason":"` + stop + `","usage":{"input_tokens":8,"output_tokens":2}}`
			}
			for _, target := range []string{"openai-chat", "openai-responses", "anthropic-messages"} {
				name := source + "/" + target + "/" + stop
				t.Run(name, func(t *testing.T) {
					r, err := ParseDialogueResult(source, []byte(raw))
					if err != nil {
						t.Fatal(err)
					}
					if limited && r.Stop != "length" {
						t.Fatalf("parser lost length: %+v", r)
					}
					out, err := EncodeDialogueResult(r, target, "public")
					if err != nil {
						t.Fatal(err)
					}
					var object map[string]any
					if json.Unmarshal(out, &object) != nil {
						t.Fatal("invalid JSON output")
					}
					if !strings.Contains(string(out), "lookup") || !strings.Contains(string(out), "prefix") {
						t.Fatalf("content lost: %s", out)
					}
					want := map[string]string{"openai-chat": `"finish_reason":"tool_calls"`, "openai-responses": `"status":"completed"`, "anthropic-messages": `"stop_reason":"tool_use"`}[target]
					if limited {
						want = map[string]string{"openai-chat": `"finish_reason":"length"`, "openai-responses": `"status":"incomplete"`, "anthropic-messages": `"stop_reason":"max_tokens"`}[target]
					}
					if !strings.Contains(string(out), want) {
						t.Fatalf("stop reason changed: %s", out)
					}
				})
			}
			if source != "anthropic-messages" {
				broken := strings.Replace(raw, `"arguments":"{}"`, `"arguments":"{"`, 1)
				if _, err := ParseDialogueResult(source, []byte(broken)); err == nil {
					t.Errorf("%s accepted truncated JSON arguments", source)
				}
			}
		}
	}
}
