package regression

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// TestMessagesEncryptedIncompleteContract 验证真实后台对合法密文 incomplete 流保留封装及 max_tokens 终态。
// 参数 t 为上下文；前置本地 SSE/隔离数据库/用户 Key，要求成功结束并正确记账；harness 清理服务、身份和日志。
func TestMessagesEncryptedIncompleteContract(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"response.reasoning_summary_text.delta\",\"delta\":\"Plan\"}\n\n")
		response := nativeAgentResponse("r_incomplete", "", true, nil)
		response["status"] = "incomplete"
		raw, _ := json.Marshal(map[string]any{"type": "response.incomplete", "response": response})
		fmt.Fprintf(w, "data: %s\n\n", raw)
	}))
	t.Cleanup(up.Close)
	dep := chatDeployment("encrypted-incomplete")
	dep.LiteLLMParams["api_base"] = up.URL + "/v1"
	dep.ModelInfo["transport"] = "bypass_openai_responses"
	dep.ModelInfo["endpoint_types"] = []string{"messages"}
	h := newHarness(t, dep)
	admin := h.adminSession()
	owner := h.openScope(t, admin, "incomplete-owner")
	reply := h.do("POST", "/v1/messages", owner.key, map[string]any{"model": dep.ModelName, "stream": true, "messages": []any{map[string]any{"role": "user", "content": "Hello"}}})
	if !strings.Contains(reply.text(), "message_stop") || !strings.Contains(reply.text(), `"stop_reason":"max_tokens"`) || !strings.Contains(reply.text(), "xhub_responses_reasoning:test-cipher") {
		t.Fatalf("合法密文 incomplete 流未保留封装和生成上限: %s", reply.describe())
	}
	h.flushSpend()
	bill := h.ok("GET", "/spend/logs/ui/"+reply.header("x-litellm-call-id"), admin, nil).json()
	if bill["status"] != "success" || bill["api_key"] != owner.keyID {
		t.Fatalf("密文日志状态或归属错误: %v", bill)
	}
}

// publicAgentResponse 构造公开推理和工具/正文组成的 Responses 回复；参数为 ID、正文和可空工具调用。
// Messages 回归和真实 CLI 夹具调用，返回固定用量对象；没有密文或厂商签名，不修改调用方对象或持久化数据。
func publicAgentResponse(id, text string, call map[string]any) map[string]any {
	response := nativeAgentResponse(id, text, false, call)
	response["output"] = append([]any{map[string]any{"id": "rs_" + id, "type": "reasoning", "summary": []any{map[string]any{"type": "summary_text", "text": "Inspect the file before answering."}}}}, response["output"].([]any)...)
	return response
}

// TestMessagesPublicReasoningContract 验证两种 OpenAI 上游经 Messages 返回推理、工具续轮、JSON/SSE 和正确账单。
// 参数 t 为上下文；真实 HTTP/独立 PostgreSQL/本地上游，不依赖外部凭据。拒绝签名与无效 Key 必须发生在外发前；harness 清理组织、用户、模型及日志。
func TestMessagesPublicReasoningContract(t *testing.T) {
	for _, protocol := range []string{"responses", "chat"} {
		t.Run(protocol, func(t *testing.T) {
			var calls atomic.Int32
			up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				calls.Add(1)
				encoded := string(mustJSON(body))
				second := strings.Contains(encoded, "file-value")
				if second && (!strings.Contains(encoded, "Inspect the file before answering.") || !strings.Contains(encoded, `"call_1"`)) {
					t.Error("Messages 工具续轮丢失公开推理或调用 ID")
				}
				var call map[string]any
				if body["tools"] != nil && !second {
					call = map[string]any{"type": "function_call", "id": "fc_1", "call_id": "call_1", "name": "read", "arguments": "{}"}
				}
				response := publicAgentResponse(fmt.Sprintf("r_%d", calls.Load()), "Done", call)
				if protocol == "responses" {
					writeAgentResponse(w, response, body["stream"] == true)
					return
				}
				message := map[string]any{"role": "assistant", "content": "Done", "reasoning_content": "Inspect the file before answering."}
				stop := "stop"
				if call != nil {
					message["content"] = nil
					message["tool_calls"] = []any{map[string]any{"id": "call_1", "type": "function", "function": map[string]any{"name": "read", "arguments": "{}"}}}
					stop = "tool_calls"
				}
				usage := map[string]any{"prompt_tokens": 8, "completion_tokens": 2}
				if body["stream"] == true {
					w.Header().Set("Content-Type", "text/event-stream")
					raw, _ := json.Marshal(map[string]any{"id": "chat_r", "choices": []any{map[string]any{"index": 0, "delta": message, "finish_reason": stop}}, "usage": usage})
					fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", raw)
				} else {
					writeJSON(w, map[string]any{"id": "chat_r", "choices": []any{map[string]any{"message": message, "finish_reason": stop}}, "usage": usage})
				}
			}))
			t.Cleanup(up.Close)
			dep := chatDeployment("grok-public-reasoning")
			dep.LiteLLMParams["api_base"] = up.URL + "/v1"
			dep.ModelInfo["transport"] = "bypass_openai_" + protocol
			dep.ModelInfo["endpoint_types"] = []string{"messages"}
			h := newHarness(t, dep)
			admin := h.adminSession()
			owner := h.openScope(t, admin, "thinking-owner")
			for _, stream := range []bool{false, true} {
				first := h.ok("POST", "/v1/messages", owner.key, map[string]any{"model": dep.ModelName, "max_tokens": 256, "stream": stream, "messages": []any{map[string]any{"role": "user", "content": "Read the file."}}, "tools": []any{map[string]any{"name": "read", "input_schema": map[string]any{"type": "object"}}}})
				if !strings.Contains(first.text(), `"thinking"`) || !strings.Contains(first.text(), "tool_use") {
					t.Fatalf("未返回 thinking 和 tool_use: %s", first.describe())
				}
				content := []any{map[string]any{"type": "thinking", "thinking": "Inspect the file before answering.", "signature": ""}, map[string]any{"type": "tool_use", "id": "call_1", "name": "read", "input": map[string]any{}}}
				second := h.ok("POST", "/v1/messages", owner.key, map[string]any{"model": dep.ModelName, "max_tokens": 256, "stream": stream, "messages": []any{map[string]any{"role": "user", "content": "Read the file."}, map[string]any{"role": "assistant", "content": content}, map[string]any{"role": "user", "content": []any{map[string]any{"type": "tool_result", "tool_use_id": "call_1", "content": "file-value"}}}}})
				if !strings.Contains(second.text(), "Done") {
					t.Fatalf("工具续轮没有最终答案: %s", second.describe())
				}
				if stream && !strings.Contains(second.text(), "message_stop") {
					t.Fatal("Messages 流没有完成事件")
				}
				h.flushSpend()
				for _, reply := range []reply{first, second} {
					bill := h.ok("GET", "/spend/logs/ui/"+reply.header("x-litellm-call-id"), admin, nil).json()
					amount, _ := floatField(bill, "spend")
					if bill["status"] != "success" || bill["api_key"] != owner.keyID || !nearlyEqual(amount, 8*testInputRate+2*testOutputRate) {
						t.Fatalf("推理请求未正确记录归属/费用: %v", bill)
					}
				}
			}
			before := calls.Load()
			bad := map[string]any{"model": dep.ModelName, "messages": []any{map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "thinking", "thinking": "Plan", "signature": "vendor-signature"}}}}}
			if response := h.do("POST", "/v1/messages", owner.key, bad); response.status != 400 {
				t.Fatalf("厂商签名未拒绝: %s", response.describe())
			}
			if response := h.do("POST", "/v1/messages", "sk-invalid", bad); response.status != 401 {
				t.Fatalf("无效 Key 未拒绝: %s", response.describe())
			}
			if calls.Load() != before {
				t.Fatal("失败输入仍调用了上游")
			}
		})
	}
}
