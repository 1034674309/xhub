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
)

// TestMessagesReasoningStreamBlocks 验证公开推理、正文与并行工具片段串行编码为完整 Messages 内容块。
// 参数 t 为上下文；内存 SSE 覆盖 Chat/Responses、截断流和空推理；要求块不重叠、不向已关闭块追加、工具参数完整且不伪造成功，无需清理。
func TestMessagesReasoningStreamBlocks(t *testing.T) {
	for _, source := range []string{"openai-chat", "openai-responses"} {
		for _, complete := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/complete=%v", source, complete), func(t *testing.T) {
				frames := []string{`{"id":"r","choices":[{"index":0,"delta":{"reasoning_content":""}}]}`, `{"choices":[{"index":0,"delta":{"reasoning_content":"Plan"}}]}`, `{"choices":[{"index":0,"delta":{"content":"Reading"}}]}`, `{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"c","function":{"name":"read","arguments":"{"}},{"index":1,"id":"d","function":{"name":"read","arguments":"{}"}}]}}]}`, `{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":8,"completion_tokens":2}}`}
				terminal := "[DONE]"
				if source == "openai-responses" {
					frames = []string{`{"type":"response.created","response":{"id":"r"}}`, `{"type":"response.reasoning_summary_text.delta","delta":"Plan"}`, `{"type":"response.output_text.delta","delta":"Reading"}`, `{"type":"response.output_item.added","output_index":2,"item":{"type":"function_call","call_id":"c","name":"read","arguments":"{}"}}`}
					terminal = `{"type":"response.completed","response":{"id":"r","status":"completed","output":[{"id":"rs","type":"reasoning","summary":[{"type":"summary_text","text":"Plan"}]},{"type":"message","content":[{"type":"output_text","text":"Reading"}]},{"type":"function_call","call_id":"c","name":"read","arguments":"{}"}],"usage":{"input_tokens":8,"output_tokens":2}}}`
				}
				if complete {
					frames = append(frames, terminal)
				}
				raw := ""
				for _, frame := range frames {
					raw += "data: " + frame + "\n\n"
				}
				w := httptest.NewRecorder()
				_, _, _, output, err := PipeDialogue(w, &http.Response{Body: io.NopCloser(strings.NewReader(raw))}, time.Now(), "public", source, "anthropic-messages")
				if (err == nil) != complete {
					t.Fatalf("Messages 流完成判定错误: %v %s", err, output)
				}
				active := -1
				thinking, tools, stops := 0, 0, 0
				for _, line := range strings.Split(string(output), "\n") {
					if !strings.HasPrefix(line, "data: ") {
						continue
					}
					var event map[string]any
					if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
						t.Fatal(err)
					}
					index := streamIndex(event["index"])
					switch event["type"] {
					case "content_block_start":
						if active != -1 {
							t.Fatalf("内容块重叠: %s", output)
						}
						active = index
						if streamMap(event["content_block"])["type"] == "thinking" {
							thinking++
						}
						if streamMap(event["content_block"])["type"] == "tool_use" {
							tools++
						}
					case "content_block_delta":
						if active != index {
							t.Fatalf("增量写入非活动块: %s", output)
						}
						if partial, ok := streamMap(event["delta"])["partial_json"].(string); ok && !json.Valid([]byte(partial)) {
							t.Fatalf("工具参数未完成即被发送: %s", partial)
						}
					case "content_block_stop":
						if active != index {
							t.Fatalf("内容块重复或错误关闭: %s", output)
						}
						active = -1
					case "message_stop":
						stops++
					}
				}
				expectedThinking := 1
				if source == "openai-responses" && !complete {
					// Responses 推理未完成时不能知道最终签名，先不发布不完整 thinking 块。
					expectedThinking = 0
				}
				if thinking != expectedThinking || (complete && (active != -1 || tools == 0 || stops != 1)) || (!complete && stops != 0) {
					t.Fatalf("thinking/工具/终态丢失: %s", output)
				}
			})
		}
	}
}

// TestMessagesReasoningTerminalBoundaries 验证 completed/incomplete 都把密文封装为不透明签名，达到上限保留 max_tokens。
// 参数 t 为上下文；本地 SSE 覆盖公开/密文终态；必须有 message_stop，密文签名独立于公开文本；纯内存无需清理。
func TestMessagesReasoningTerminalBoundaries(t *testing.T) {
	for _, status := range []string{"completed", "incomplete"} {
		for _, encrypted := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/encrypted=%v", status, encrypted), func(t *testing.T) {
				raw := "data: {\"type\":\"response.reasoning_summary_text.delta\",\"delta\":\"Plan\"}\n\n" + fmt.Sprintf("data: {\"type\":\"response.%s\",\"response\":{\"id\":\"r\",\"status\":\"%s\",\"output\":[{\"id\":\"rs\",\"type\":\"reasoning\",\"encrypted_content\":\"cipher\",\"summary\":[{\"type\":\"summary_text\",\"text\":\"Plan\"}]}]}}\n\n", status, status)
				if !encrypted {
					raw = strings.ReplaceAll(raw, `"encrypted_content":"cipher",`, "")
				}
				_, _, _, out, err := PipeDialogue(httptest.NewRecorder(), &http.Response{Body: io.NopCloser(strings.NewReader(raw))}, time.Now(), "public", "openai-responses", "anthropic-messages")
				if encrypted && !strings.Contains(string(out), `"signature":"xhub_responses_reasoning:cipher"`) {
					t.Fatalf("密文终态 %s 丢失封装签名: %v %s", status, err, out)
				}
				if err != nil || !strings.Contains(string(out), "message_stop") || (status == "incomplete" && !strings.Contains(string(out), `"stop_reason":"max_tokens"`)) {
					t.Fatalf("推理终态失败: %v %s", err, out)
				}
			})
		}
	}
}
