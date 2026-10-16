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

// TestChatReasoningStreamTerminal 验证 Chat 在完成/截断终态保留推理扩展、工具、用量和提前提交的归属。
// 参数 t 为上下文；内存 SSE 使用稀疏工具索引及终态密文，断言只发布完整扩展，失败流不提交或伪造 DONE；无需外部清理。
func TestChatReasoningStreamTerminal(t *testing.T) {
	for _, mode := range []string{"completed", "incomplete", "truncated", "failed", "bad-cipher"} {
		t.Run(mode, func(t *testing.T) {
			frames := []string{`{"type":"response.created","response":{"id":"resp"}}`, `{"type":"response.reasoning_summary_text.delta","delta":"Plan"}`, `{"type":"response.output_item.added","output_index":2,"item":{"type":"function_call","call_id":"call","name":"read","arguments":"{}"}}`}
			status := mode
			if mode == "bad-cipher" {
				status = "completed"
			}
			if mode == "failed" {
				frames = append(frames, `{"type":"response.failed"}`)
			} else if mode != "truncated" {
				terminal := fmt.Sprintf(`{"type":"response.%s","response":{"id":"resp","status":"%s","output":[{"id":"rs","type":"reasoning","summary":[{"type":"summary_text","text":"Plan"}],"encrypted_content":"terminal-cipher"},{"type":"function_call","call_id":"call","name":"read","arguments":"{}"}],"usage":{"input_tokens":8,"output_tokens":2}}}`, status, status)
				if mode == "bad-cipher" {
					terminal = strings.Replace(terminal, `"terminal-cipher"`, `42`, 1)
				}
				frames = append(frames, terminal)
			}
			raw := ""
			for _, frame := range frames {
				raw += "data: " + frame + "\n\n"
			}
			writer := httptest.NewRecorder()
			commits := 0
			_, usage, _, output, err := PipeDialogue(writer, &http.Response{Body: io.NopCloser(strings.NewReader(raw))}, time.Now(), "public", "openai-responses", "openai-chat", func(result llmdialogue.DialogueResult) {
				commits++
				if result.ID != "resp" || strings.Contains(writer.Body.String(), `"reasoning_items"`) {
					t.Error("Chat 完成前归属提交顺序错误")
				}
			})
			valid := mode == "completed" || mode == "incomplete"
			if (err == nil) != valid || (commits == 1) != valid || strings.Contains(string(output), "[DONE]") != valid {
				t.Fatalf("Chat 推理流终态错误: %v %s", err, output)
			}
			if !valid {
				if strings.Contains(string(output), "terminal-cipher") {
					t.Fatal("失败流泄漏了未验证的推理扩展")
				}
				return
			}
			count := 0
			for _, line := range strings.Split(string(output), "\n") {
				if !strings.HasPrefix(line, "data: {") {
					continue
				}
				var event map[string]any
				_ = json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event)
				for _, value := range llmdialogueList(event["choices"]) {
					choice := streamMap(value)
					delta := streamMap(choice["delta"])
					if items, ok := delta["reasoning_items"].([]any); ok {
						count++
						if streamMap(items[0])["encrypted_content"] != "terminal-cipher" || streamMap(items[0])["id"] != "rs" {
							t.Fatalf("Chat 推理扩展丢失: %s", output)
						}
						if mode == "incomplete" && choice["finish_reason"] != "length" {
							t.Fatal("Chat 截断原因未保留")
						}
					}
				}
			}
			if count != 1 || usage["completion_tokens"] != float64(2) || !strings.Contains(string(output), `"index":0`) {
				t.Fatalf("Chat 工具索引、推理终态或用量错误: %s", output)
			}
		})
	}
}

// llmdialogueList 为流测试安全读取可选数组；参数为 JSON 值，返回数组或 nil，无副作用与清理。
func llmdialogueList(value any) []any { items, _ := value.([]any); return items }
