package regression

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestStreamMissingSemanticTerminal 验证缺少协议语义终态的 SSE 不得被网关标记成功或发布可续接历史。
// 参数 t 是测试上下文；前置真实网关、私有 PostgreSQL schema 和本地异常上游；验证响应/日志/续接，harness 自动清理所有数据。
func TestStreamMissingSemanticTerminal(t *testing.T) {
	for _, tc := range []struct{ name, transport, endpoint, raw string }{
		{"chat_to_responses_missing_finish", "chat", "responses", `data: {"id":"audit_missing_finish","choices":[{"index":0,"delta":{"content":"partial"}}],"usage":{"prompt_tokens":8,"completion_tokens":2}}` + "\n\ndata: [DONE]\n\n"},
		{"chat_to_messages_missing_finish", "chat", "messages", `data: {"id":"audit_missing_finish","choices":[{"index":0,"delta":{"content":"partial"}}],"usage":{"prompt_tokens":8,"completion_tokens":2}}` + "\n\ndata: [DONE]\n\n"},
		{"chat_native_missing_finish_control", "chat", "chat/completions", `data: {"id":"audit_missing_finish","choices":[{"index":0,"delta":{"content":"partial"}}],"usage":{"prompt_tokens":8,"completion_tokens":2}}` + "\n\ndata: [DONE]\n\n"},
		{"responses_to_chat_missing_response", "responses", "chat/completions", `data: {"type":"response.created","response":{"id":"audit_missing_finish"}}` + "\n\n" + `data: {"type":"response.output_text.delta","delta":"partial"}` + "\n\n" + `data: {"type":"response.completed"}` + "\n\n"},
		{"responses_to_messages_missing_response", "responses", "messages", `data: {"type":"response.created","response":{"id":"audit_missing_finish"}}` + "\n\n" + `data: {"type":"response.output_item.added","output_index":0,"item":{"type":"message","role":"assistant","content":[]}}` + "\n\n" + `data: {"type":"response.output_text.delta","delta":"partial"}` + "\n\n" + `data: {"type":"response.completed"}` + "\n\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var count atomic.Int32
			up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count.Add(1)
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				if body["stream"] == true {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprint(w, tc.raw)
					return
				}
				writeJSON(w, chatAnswer("vendor", nil))
			}))
			t.Cleanup(up.Close)
			dep := chatDeployment("audit-terminal")
			dep.LiteLLMParams["api_base"] = up.URL + "/v1"
			dep.ModelInfo["id"] = "terminal-regression-dep"
			dep.ModelInfo["transport"] = "bypass_openai_" + tc.transport
			dep.ModelInfo["endpoint_types"] = []string{"responses", "messages", "chat"}
			h := newHarness(t, dep)
			admin := h.adminSession()
			owner := h.openScope(t, admin, "audit-terminal-owner")
			body := map[string]any{"model": dep.ModelName, "stream": true, "messages": []any{map[string]any{"role": "user", "content": "Reply with a complete answer."}}}
			if tc.endpoint == "responses" {
				delete(body, "messages")
				body["input"] = "Reply with a complete answer."
			}
			reply := h.do("POST", "/v1/"+tc.endpoint, owner.key, body)
			h.flushSpend()
			bill := h.ok("GET", "/spend/logs/ui/"+reply.header("x-litellm-call-id"), admin, nil).json()
			t.Logf("EVIDENCE status=%d upstream_calls=%d log_status=%v spend=%v response=%s", reply.status, count.Load(), bill["status"], bill["spend"], reply.text())
			if tc.endpoint == "responses" {
				next := h.do("POST", "/v1/responses", owner.key, map[string]any{"model": dep.ModelName, "input": "Continue", "previous_response_id": "audit_missing_finish"})
				t.Logf("EVIDENCE continuation_status=%d upstream_calls_after=%d response=%s", next.status, count.Load(), next.text())
				if next.status != 400 || count.Load() != 1 {
					t.Error("失败流发布续接历史或再次外发")
				}
			}
			if bill["status"] != "error" || parseFloatOrZero(fmt.Sprint(bill["spend"])) != 0 || count.Load() != 1 {
				t.Fatalf("失败流的状态、费用或重试不正确: bill=%v count=%d", bill, count.Load())
			}
			if strings.Contains(reply.text(), `"status":"completed"`) || strings.Contains(reply.text(), `"stop_reason":"end_turn"`) || strings.Contains(reply.text(), `"finish_reason":"stop"`) {
				t.Error("缺少语义终态的流生成了正常完成事件")
			}
		})
	}
}

// TestTruncatedToolStopMatrix 验证完整工具参数伴随输出限额终态时仍应保留截断状态，不得伪造正常工具结束。
// 参数 t 是上下文；真实 HTTP/私有数据库/本地上游覆盖 Chat 与 Responses 的 JSON/SSE；自动清理 schema 和服务。
func TestTruncatedToolStopMatrix(t *testing.T) {
	for _, transport := range []string{"chat", "responses"} {
		for _, endpoint := range []string{"responses", "messages", "chat/completions"} {
			for _, stream := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s_to_%s_stream_%t", transport, endpoint, stream), func(t *testing.T) {
					up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						call := map[string]any{"type": "function_call", "id": "fc_audit", "call_id": "call_audit", "name": "read", "arguments": "{}"}
						if transport == "responses" {
							response := nativeAgentResponse("audit_length", "", false, call)
							response["status"] = "incomplete"
							response["incomplete_details"] = map[string]any{"reason": "max_output_tokens"}
							if !stream {
								writeJSON(w, response)
								return
							}
							w.Header().Set("Content-Type", "text/event-stream")
							added := map[string]any{"type": "response.output_item.added", "output_index": 0, "item": call}
							raw, _ := json.Marshal(added)
							fmt.Fprintf(w, "data: %s\n\n", raw)
							raw, _ = json.Marshal(map[string]any{"type": "response.incomplete", "response": response})
							fmt.Fprintf(w, "data: %s\n\n", raw)
							return
						}
						msg := map[string]any{"role": "assistant", "content": nil, "tool_calls": []any{map[string]any{"index": 0, "id": "call_audit", "type": "function", "function": map[string]any{"name": "read", "arguments": "{}"}}}}
						choice := map[string]any{"index": 0, "finish_reason": "length"}
						if stream {
							choice["delta"] = msg
						} else {
							choice["message"] = msg
						}
						response := map[string]any{"id": "audit_length", "object": "chat.completion", "model": "vendor", "choices": []any{choice}, "usage": map[string]any{"prompt_tokens": 8, "completion_tokens": 2}}
						if !stream {
							writeJSON(w, response)
							return
						}
						w.Header().Set("Content-Type", "text/event-stream")
						raw, _ := json.Marshal(response)
						fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", raw)
					}))
					t.Cleanup(up.Close)
					dep := chatDeployment("audit-length")
					dep.LiteLLMParams["api_base"] = up.URL + "/v1"
					dep.ModelInfo["transport"] = "bypass_openai_" + transport
					dep.ModelInfo["endpoint_types"] = []string{"responses", "messages", "chat"}
					h := newHarness(t, dep)
					admin := h.adminSession()
					owner := h.openScope(t, admin, "audit-length-owner")
					body := map[string]any{"model": dep.ModelName, "stream": stream, "messages": []any{map[string]any{"role": "user", "content": "Read the file."}}}
					if endpoint == "responses" {
						delete(body, "messages")
						body["input"] = "Read the file."
					}
					reply := h.do("POST", "/v1/"+endpoint, owner.key, body)
					h.flushSpend()
					bill := h.ok("GET", "/spend/logs/ui/"+reply.header("x-litellm-call-id"), admin, nil).json()
					if bill["status"] != "success" || parseFloatOrZero(fmt.Sprint(bill["spend"])) <= 0 {
						t.Fatalf("合法截断终态未按实际usage唯一结算: %v", bill)
					}
					t.Logf("EVIDENCE status=%d response=%s", reply.status, reply.text())
					expected := map[string]string{"responses": `"status":"incomplete"`, "messages": `"stop_reason":"max_tokens"`, "chat/completions": `"finish_reason":"length"`}[endpoint]
					if reply.status != 200 || !strings.Contains(reply.text(), expected) {
						t.Errorf("工具回复丢失输出限额终态，缺少 %s", expected)
					}
				})
			}
		}
	}
}

// streamTerminalFrames 构造三个协议各自完整、带usage的合法终态流；供EOF边界测试使用，不依赖外部模型。
func streamTerminalFrames(protocol string) string {
	switch protocol {
	case "chat":
		return "data: {\"id\":\"chatcmpl_terminal\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"TERMINAL_OK\"}}]}\n\ndata: {\"id\":\"chatcmpl_terminal\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":8,\"completion_tokens\":2}}\n\ndata: [DONE]\n\n"
	case "responses":
		return "event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_terminal\",\"status\":\"in_progress\",\"output\":[]}}\n\nevent: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"item_id\":\"msg_terminal\",\"output_index\":0,\"content_index\":0,\"delta\":\"TERMINAL_OK\"}\n\nevent: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_terminal\",\"object\":\"response\",\"status\":\"completed\",\"output\":[{\"id\":\"msg_terminal\",\"type\":\"message\",\"status\":\"completed\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"TERMINAL_OK\",\"annotations\":[]}]}],\"usage\":{\"input_tokens\":8,\"output_tokens\":2}}}\n\n"
	default:
		return "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_terminal\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"usage\":{\"input_tokens\":8,\"output_tokens\":0}}}\n\nevent: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\nevent: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"TERMINAL_OK\"}}\n\nevent: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\nevent: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":2}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	}
}

// TestStreamTerminalWithoutEOF 验证明确合法终态应立即结束，不依赖上游关闭HTTP正文。
// 参数t为测试上下文；8个独立schema场景使用本地SSE，保留正文连接或正常EOF作对照；全部禁用Redis和付费上游。
// route timeout为250ms，上游最多保留2秒；每例核对客户端终态、唯一外发、账单和终止延迟，服务与schema自动回收。
func TestStreamTerminalWithoutEOF(t *testing.T) {
	t.Setenv("XHUB_REGRESSION_REDIS_URL", "")

	cases := []struct {
		up, caller string
		hold       bool
	}{{"chat", "chat", true}, {"chat", "responses", true}, {"chat", "messages", true}, {"responses", "responses", true}, {"responses", "chat", true}, {"responses", "messages", true}, {"messages", "messages", true}, {"chat", "responses", false}}
	for _, tc := range cases {
		name := fmt.Sprintf("%s_to_%s_hold_%v", tc.up, tc.caller, tc.hold)
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				_, _ = io.Copy(io.Discard, r.Body)
				w.Header().Set("Content-Type", "text/event-stream")
				io.WriteString(w, streamTerminalFrames(tc.up))
				w.(http.Flusher).Flush()
				if tc.hold {
					select {
					case <-r.Context().Done():
					case <-time.After(2 * time.Second):
					}
				}
			}))
			t.Cleanup(upstream.Close)
			model := "round3-terminal-" + name
			dep := chatDeployment(model)
			dep.LiteLLMParams["api_base"] = upstream.URL
			dep.ModelInfo["id"] = model + "-dep"
			dep.ModelInfo["endpoint_types"] = []string{"chat", "responses", "messages"}
			if tc.up == "responses" {
				dep.ModelInfo["transport"] = "bypass_openai_responses"
			}
			if tc.up == "messages" {
				dep.ModelInfo["transport"] = "bypass_anthropic_messages"
				dep.LiteLLMParams["custom_llm_provider"] = "anthropic"
			}
			h := newHarness(t, dep)
			admin := h.adminSession()
			owner := h.openScope(t, admin, "terminal-owner")
			template := routeTemplate(t, h, admin, "short deadline", routeTemplateBody([]any{}, 3, 0.25, 0, 0))
			bindTemplate(t, h, admin, "key", owner.keyID, template)
			path := "/v1/chat/completions"
			body := chatRequest(model, "Return TERMINAL_OK")
			if tc.caller == "responses" {
				path = "/v1/responses"
				body = map[string]any{"model": model, "input": "Return TERMINAL_OK"}
			}
			if tc.caller == "messages" {
				path = "/v1/messages"
				body["max_tokens"] = 128
			}
			body["stream"] = true
			start := time.Now()
			reply := h.doHeaders("POST", path, owner.key, body, map[string]string{"X-Session-Id": "round3-terminal-session"})
			elapsed := time.Since(start).Seconds()
			h.flushSpend()
			bill := h.ok("GET", "/spend/logs/ui/"+reply.header("x-litellm-call-id"), admin, nil).json()
			rows := []map[string]any{bill}
			success := false
			for _, r := range rows {
				if r["status"] == "success" {
					success = true
				}
			}
			t.Logf("terminal upstream=%s caller=%s hold=%v elapsed=%.3f body=%s", tc.up, tc.caller, tc.hold, elapsed, reply.text())
			if !success || strings.Contains(reply.text(), "upstream_error") {
				t.Errorf("valid terminal became failure while waiting for EOF: success=%v seconds=%.3f", success, elapsed)
			}
			if calls.Load() != 1 {
				t.Errorf("terminal request retried: %d", calls.Load())
			}
		})
	}
}
