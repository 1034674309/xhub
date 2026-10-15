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

// TestNativeResponsesStreamAndHistory 验证完整推理 SSE 与密文历史恢复、完成回调时序及截断失败。
// 参数 t 为单测上下文；前置内存原生 SSE，终态之前提交带原始 output 的历史；无外部资源需清理。
func TestNativeResponsesStreamAndHistory(t *testing.T) {
	response := `{"id":"resp_native","status":"completed","model":"real","output":[{"type":"reasoning","id":"rs_1","summary":[{"type":"summary_text","text":"Plan"}],"encrypted_content":"cipher"},{"type":"function_call","id":"fc_1","call_id":"call_1","name":"read","arguments":"{}"}],"usage":{"input_tokens":8,"output_tokens":2,"output_tokens_details":{"reasoning_tokens":1}}}`
	prefix := "event: response.reasoning_summary_text.delta\ndata: {\"type\":\"response.reasoning_summary_text.delta\",\"sequence_number\":19,\"output_index\":0,\"delta\":\"Plan\"}\n\n"
	full := prefix + "event: response.completed\ndata: {\"type\":\"response.completed\",\"sequence_number\":20,\"response\":" + response + "}\n\n"
	for _, good := range []bool{true, false} {
		raw := prefix
		if good {
			raw = full
		}
		w := httptest.NewRecorder()
		committed := 0
		resp := &http.Response{Body: io.NopCloser(strings.NewReader(raw))}
		wrote, usage, _, captured, err := PipeDialogue(w, resp, time.Now(), "public", "openai-responses", "openai-responses", func(result llmdialogue.DialogueResult) {
			committed++
			if strings.Contains(w.Body.String(), "response.completed") || result.Native == nil {
				t.Fatal("完成回调晚于可见终态或原始项丢失")
			}
			d, e := llmdialogue.ParseDialogue("openai-responses", map[string]any{"input": "Read the file", "instructions": "This round only"})
			if e != nil {
				t.Fatal(e)
			}
			history := ResponseHistory(d, map[string]any{"instructions": "This round only"}, result)
			next, e := ResponsesRequestBody(map[string]any{"input": []any{map[string]any{"type": "function_call_output", "call_id": "call_1", "output": "value"}}, "previous_response_id": "resp_native"}, history)
			if e != nil {
				t.Fatal(e)
			}
			encoded, _ := json.Marshal(next)
			if !strings.Contains(string(encoded), "cipher") || strings.Contains(string(encoded), "This round only") {
				t.Fatalf("密文历史/临时指令错误: %s", encoded)
			}
		})
		if !wrote || (err == nil) != good || (good && committed != 1) || (!good && committed != 0) {
			t.Fatalf("终态校验错误: wrote=%v commit=%d err=%v", wrote, committed, err)
		}
		if good && (usage["total_tokens"] != float64(10) || !strings.Contains(string(captured), `"sequence_number":19`) || !strings.Contains(string(captured), "cipher")) {
			t.Fatalf("原生流信息丢失: %s %v", captured, usage)
		}
	}
}

// TestDeepSeekReasoningStreamBridge 验证 Chat 公开推理增量转换成 Responses 独立输出项及最终摘要。
// 参数 t 为上下文；前置内存 DeepSeek 兼容 SSE，最终文本/推理和用量均保留；EOF 错误由原生测试覆盖，无需清理。
func TestDeepSeekReasoningStreamBridge(t *testing.T) {
	raw := "data: {\"id\":\"chat_deepseek\",\"choices\":[{\"index\":0,\"delta\":{\"reasoning_content\":\"Plan\"}}]}\n\ndata: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Done\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":8,\"completion_tokens\":2}}\n\ndata: [DONE]\n\n"
	w := httptest.NewRecorder()
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(raw))}
	_, _, _, captured, err := PipeDialogue(w, resp, time.Now(), "public", "openai-chat", "openai-responses", func(result llmdialogue.DialogueResult) {
		if result.Reasoning != "Plan" || result.Text != "Done" {
			t.Fatalf("桥接终态丢失推理: %+v", result)
		}
	})
	if err != nil || !strings.Contains(string(captured), "response.reasoning_summary_text.delta") || !strings.Contains(string(captured), "response.reasoning_summary_part.done") {
		t.Fatalf("DeepSeek 推理流失败: %s %v", captured, err)
	}
}
