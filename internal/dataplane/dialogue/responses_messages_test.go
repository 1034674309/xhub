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

// TestResponsesMessagesEnvelopeStream 验证推理完成项、终态补齐、无摘要密文及重复终态的真实 SSE 顺序。
// 参数 t 为上下文；前置内存流，要求封装在块关闭前发送、正文不重复、usage/终态完整；无服务或数据需要清理。
func TestResponsesMessagesEnvelopeStream(t *testing.T) {
	for _, summary := range []bool{false, true} {
		for _, done := range []bool{false, true} {
			t.Run(fmt.Sprintf("summary=%v/item_done=%v", summary, done), func(t *testing.T) {
				parts := []any{}
				if summary {
					parts = append(parts, map[string]any{"type": "summary_text", "text": "Plan"})
				}
				item := map[string]any{"type": "reasoning", "id": "rs", "summary": parts, "encrypted_content": "cipher"}
				frames := []map[string]any{{"type": "response.created", "response": map[string]any{"id": "r"}},
					{"type": "response.output_item.added", "item": map[string]any{"id": "rs", "type": "reasoning"}},
					{"type": "response.output_text.delta", "delta": "Done"}}
				if done {
					frames = append(frames, map[string]any{"type": "response.output_item.done", "item": item})
				}
				frames = append(frames, map[string]any{"type": "response.completed", "response": map[string]any{
					"id": "r", "status": "completed", "output": []any{item, map[string]any{"type": "message", "content": []any{map[string]any{"type": "output_text", "text": "Done"}}}},
					"usage": map[string]any{"input_tokens": 12, "output_tokens": 5},
				}})
				var raw strings.Builder
				for _, frame := range frames {
					value, _ := json.Marshal(frame)
					fmt.Fprintf(&raw, "data: %s\n\n", value)
				}
				writer := httptest.NewRecorder()
				commits := 0
				_, _, _, out, err := PipeDialogueWithTools(writer, &http.Response{Body: io.NopCloser(strings.NewReader(raw.String()))}, time.Now(), "public", "openai-responses", "anthropic-messages", nil, func(_ llmdialogue.DialogueResult) {
					commits++
					if strings.Contains(writer.Body.String(), `"type":"message_stop"`) {
						t.Error("客户端可见工具终态之前尚未提交连接归属")
					}
				})
				if err != nil {
					t.Fatal(err)
				}
				if commits != 1 {
					t.Fatalf("Messages 完成前归属提交次数错误: %d", commits)
				}
				text := string(out)
				if strings.Count(text, "xhub_responses_reasoning:cipher") != 1 || strings.Count(text, `"text":"Done"`) != 1 || !strings.Contains(text, `"input_tokens":12`) || !strings.Contains(text, `"output_tokens":5`) || strings.Count(text, `"type":"message_stop"`) != 1 {
					t.Fatalf("推理封装、正文或用量重复/丢失: %s", out)
				}
				if summary && strings.Index(text, `"type":"signature_delta"`) > strings.Index(text, `"type":"content_block_stop"`) {
					t.Fatalf("签名被写入已关闭块: %s", out)
				}
				if !summary && !strings.Contains(text, `"type":"redacted_thinking"`) {
					t.Fatalf("无摘要密文没有独立块: %s", out)
				}
			})
		}
	}
}

// TestResponsesMessagesMalformedReasoning 验证损坏密文和前后矛盾的完成项不会伪装成功。
// 参数 t 为上下文；前置无外部依赖的 SSE，所有失败必须缺少 message_stop；内存对象随用例回收。
func TestResponsesMessagesMalformedReasoning(t *testing.T) {
	for _, cipher := range []string{`42`, `false`, `{}`} {
		raw := `data: {"type":"response.reasoning_summary_text.delta","delta":"Plan"}` + "\n\n" +
			`data: {"type":"response.completed","response":{"id":"r","status":"completed","output":[{"id":"rs","type":"reasoning","summary":[],"encrypted_content":` + cipher + `}]}}` + "\n\n"
		_, _, _, out, err := PipeDialogue(httptest.NewRecorder(), &http.Response{Body: io.NopCloser(strings.NewReader(raw))}, time.Now(), "public", "openai-responses", "anthropic-messages")
		if err == nil || strings.Contains(string(out), "message_stop") {
			t.Fatalf("非法密文变成成功流: %v %s", err, out)
		}
	}
	raw := "data: " + `{"type":"response.output_item.done","item":{"id":"rs","type":"reasoning","summary":[],"encrypted_content":"one"}}` + "\n\n" +
		"data: " + `{"type":"response.completed","response":{"id":"r","status":"completed","output":[{"id":"rs","type":"reasoning","summary":[],"encrypted_content":"two"}]}}` + "\n\n"
	_, _, _, out, err := PipeDialogue(httptest.NewRecorder(), &http.Response{Body: io.NopCloser(strings.NewReader(raw))}, time.Now(), "public", "openai-responses", "anthropic-messages")
	if err == nil || !strings.Contains(err.Error(), "conflicting completed reasoning item") || strings.Contains(string(out), "message_stop") {
		t.Fatalf("完成项与终态密文冲突未被拒绝: %v %s", err, out)
	}
}
