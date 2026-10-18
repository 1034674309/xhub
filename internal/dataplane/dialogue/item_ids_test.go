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
		if len(seen) != 2 || seen[llmdialogue.ResponseTextItemID("msg", first)] < 4 || seen[llmdialogue.ResponseTextItemID("rs", first)] < 4 {
			t.Fatalf("SSE item ID 未保持一致，late=%v: %v", late, seen)
		}
	}
}
