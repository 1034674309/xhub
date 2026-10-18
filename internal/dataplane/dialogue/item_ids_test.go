package dialogue

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	llmdialogue "github.com/sunqirui1987/xhub/internal/llm/dialogue"
)

// TestResponsesStreamTextItemIDs 验证 Chat→Responses SSE 各生命周期共享合法且稳定的 ID；覆盖缺少首帧 ID/末帧变更，内存流无清理。
func TestResponsesStreamTextItemIDs(t *testing.T) {
	for _, late := range []bool{false, true} {
		first := "chatcmpl-first"
		if late {
			first = ""
		}
		chunks := []map[string]any{{"id": first, "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"reasoning_content": "Plan", "content": "O"}}}}, {"id": "chatcmpl-final", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "K"}, "finish_reason": "stop"}}, "usage": map[string]any{"prompt_tokens": 8, "completion_tokens": 2}}}
		var input strings.Builder
		for _, c := range chunks {
			raw, _ := json.Marshal(c)
			input.WriteString("data: " + string(raw) + "\n\n")
		}
		input.WriteString("data: [DONE]\n\n")
		writer := httptest.NewRecorder()
		_, _, _, _, err := PipeDialogue(writer, &http.Response{Body: io.NopCloser(strings.NewReader(input.String()))}, time.Now(), "public", "openai-chat", "openai-responses")
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]int{}
		for _, line := range strings.Split(writer.Body.String(), "\n") {
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var event map[string]any
			if err = json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
				t.Fatal(err)
			}
			if id, ok := event["item_id"].(string); ok {
				seen[id]++
			}
			if item, ok := event["item"].(map[string]any); ok {
				seen[item["id"].(string)]++
			}
			if event["type"] == "response.completed" {
				r := event["response"].(map[string]any)
				for _, value := range r["output"].([]any) {
					seen[value.(map[string]any)["id"].(string)]++
				}
			}
		}
		if len(seen) != 2 || seen[llmdialogue.ResponseItemID("msg", first)] < 4 || seen[llmdialogue.ResponseItemID("rs", first)] < 4 {
			t.Fatalf("SSE item ID 未保持一致，late=%v: %v", late, seen)
		}
	}
}

// TestResponsesStreamFunctionItemIDs 验证未缓冲/已缓冲 Chat 函数回复的 added/delta/done/completed 都使用同一 fc_，关联 call_id 不变。
// 前置内存 SSE、普通与 namespace 工具映射；参数缺失仍由原状态机拒绝，无网络或数据清理。
func TestResponsesStreamFunctionItemIDs(t *testing.T) {
	raw := "data: {\"id\":\"chatcmpl-tools\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call-old\",\"type\":\"function\",\"function\":{\"name\":\"lookup_value\",\"arguments\":\"{}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n"
	for _, buffered := range []bool{false, true} {
		var bindings map[string]llmdialogue.ToolBinding
		if buffered {
			bindings = map[string]llmdialogue.ToolBinding{"lookup_value": {Name: "lookup_value", Namespace: "lookup"}}
		}
		_, _, _, out, err := PipeDialogueWithTools(httptest.NewRecorder(), &http.Response{Body: io.NopCloser(strings.NewReader(raw))}, time.Now(), "public", "openai-chat", "openai-responses", bindings)
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		want := llmdialogue.ResponseItemID("fc", "call-old")
		for _, line := range strings.Split(string(out), "\n") {
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var e map[string]any
			if err = json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &e); err != nil {
				t.Fatal(err)
			}
			if id, ok := e["item_id"].(string); ok {
				seen++
				if id != want {
					t.Fatalf("函数增量 ID 与终态不同: %v", e)
				}
			}
			items := []any{}
			if item := streamMap(e["item"]); item != nil {
				items = append(items, item)
			}
			if e["type"] == "response.completed" {
				items = streamMap(e["response"])["output"].([]any)
			}
			for _, value := range items {
				item := streamMap(value)
				seen++
				if item["id"] != want || item["call_id"] != "call-old" {
					t.Fatalf("函数生命周期 ID 或工具关联错误: %v", item)
				}
			}
		}
		if seen < 4 {
			t.Fatalf("缺少函数 ID 生命周期，buffered=%v: %s", buffered, out)
		}
	}
}
