package dialogue

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	llmdialogue "github.com/sunqirui1987/xhub/internal/llm/dialogue"
)

// TestAgentToolStreamRestore 验证真实 SSE 解码后还原 namespace/custom 工具及完整参数，失败时不得发布成功终态。
// 参数 t 为上下文；前置内存 HTTP 响应和分片 JSON；PipeDialogue 关闭响应体，无外部服务或数据需要清理。
func TestAgentToolStreamRestore(t *testing.T) {
	for _, tc := range []struct {
		name, arguments string
		custom, valid   bool
	}{
		{"function", `{}`, false, true}, {"custom", `{"content":"line 1\nline 2"}`, true, true},
		{"invalid_custom", `{"content":42}`, true, false}, {"truncated", `{"content":`, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var raw strings.Builder
			for _, chunk := range []map[string]any{
				{"id": "r", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"tool_calls": []any{map[string]any{"index": 3, "id": "call_1", "type": "function", "function": map[string]any{"name": "fs__read", "arguments": tc.arguments[:len(tc.arguments)/2]}}}}, "finish_reason": nil}}},
				{"id": "r", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"tool_calls": []any{map[string]any{"index": 3, "function": map[string]any{"arguments": tc.arguments[len(tc.arguments)/2:]}}}}, "finish_reason": "tool_calls"}}},
				{"id": "r", "choices": []any{}, "usage": map[string]any{"prompt_tokens": 8, "completion_tokens": 2}},
			} {
				encoded, _ := json.Marshal(chunk)
				fmt.Fprintf(&raw, "data: %s\n\n", encoded)
			}
			raw.WriteString("data: [DONE]\n\n")
			bindings := map[string]llmdialogue.ToolBinding{"fs__read": {Name: "read", Namespace: "fs", Custom: tc.custom}}
			_, _, _, output, err := PipeDialogueWithTools(httptest.NewRecorder(), &http.Response{Body: io.NopCloser(strings.NewReader(raw.String()))}, time.Now(), "public", "openai-chat", "openai-responses", bindings)
			if (err == nil) != tc.valid {
				t.Fatalf("工具流状态与参数有效性不符: %v, %s", err, output)
			}
			if !tc.valid {
				if strings.Contains(string(output), "response.completed") || strings.Contains(string(output), "response.custom_tool_call_input.delta") {
					t.Fatalf("非法 custom 参数被发布给客户端执行: %s", output)
				}
				return
			}
			if !strings.Contains(string(output), `"namespace":"fs"`) || !strings.Contains(string(output), `"name":"read"`) || !strings.Contains(string(output), "response.completed") {
				t.Fatalf("工具身份或终态丢失: %s", output)
			}
			if tc.custom && (!strings.Contains(string(output), "response.custom_tool_call_input.delta") || !strings.Contains(string(output), `"input":"line 1\nline 2"`)) {
				t.Fatalf("custom 输入未从 JSON 包装还原: %s", output)
			}
		})
	}
}

// TestMessagesStreamLateUsage 验证 Chat 最后独立 usage 帧被完整回填给 Claude，而不是只返回输出 Token。
// 参数 t 为上下文；前置含终止帧、后置统计的真实 SSE 形状，检查开始为数值零、结束为实测数值；仅内存响应，无持久化清理。
func TestMessagesStreamLateUsage(t *testing.T) {
	raw := "data: {\"id\":\"r\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"OK\"},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":8,\"completion_tokens\":2,\"prompt_tokens_details\":{\"cached_tokens\":0}}}\n\ndata: [DONE]\n\n"
	_, usage, _, output, err := PipeDialogue(httptest.NewRecorder(), &http.Response{Body: io.NopCloser(strings.NewReader(raw))}, time.Now(), "public", "openai-chat", "anthropic-messages")
	if err != nil {
		t.Fatal(err)
	}
	seenStart, seenFinal := false, false
	for _, line := range strings.Split(string(output), "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event map[string]any
		_ = json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event)
		if event["type"] == "message_start" {
			seenStart = true
			if streamMap(streamMap(event["message"])["usage"])["input_tokens"] != float64(0) {
				t.Fatal("初始未知输入不能返回 null 或伪造实测数值")
			}
		}
		if event["type"] == "message_delta" {
			seenFinal = true
			u := streamMap(event["usage"])
			if u["input_tokens"] != float64(8) || u["output_tokens"] != float64(2) || u["cache_read_input_tokens"] != float64(0) {
				t.Fatalf("最终统计未补齐: %v", u)
			}
		}
	}
	if !seenStart || !seenFinal || usage["prompt_tokens"] != float64(8) || usage["completion_tokens"] != float64(2) {
		t.Fatalf("生命周期或内部记账事实丢失: %s, %v", output, usage)
	}
}
