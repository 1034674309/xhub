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

// chatReasoningMessage 按真实 Chat 客户端方式汇总 JSON/SSE 的助手消息，保留扩展、文本和工具参数。
// 参数为测试上下文、HTTP 回复和流开关，返回可直接加入历史的消息；仅处理本地测试响应，不产生存储副作用。
func chatReasoningMessage(t *testing.T, response reply, stream bool) map[string]any {
	t.Helper()
	if !stream {
		return response.json()["choices"].([]any)[0].(map[string]any)["message"].(map[string]any)
	}
	message := map[string]any{"role": "assistant", "content": ""}
	calls := map[int]map[string]any{}
	for _, line := range strings.Split(response.text(), "\n") {
		if !strings.HasPrefix(line, "data: {") {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
			t.Fatal(err)
		}
		choices, _ := event["choices"].([]any)
		for _, value := range choices {
			delta := value.(map[string]any)["delta"].(map[string]any)
			for _, key := range []string{"content", "reasoning_content"} {
				if text, ok := delta[key].(string); ok {
					message[key] = stringField(message, key) + text
				}
			}
			if items, ok := delta["reasoning_items"].([]any); ok {
				message["reasoning_items"] = items
			}
			tools, _ := delta["tool_calls"].([]any)
			for _, value := range tools {
				tool := value.(map[string]any)
				index := int(tool["index"].(float64))
				call := calls[index]
				if call == nil {
					call = map[string]any{"type": "function", "function": map[string]any{"name": "", "arguments": ""}}
					calls[index] = call
				}
				if id, ok := tool["id"].(string); ok {
					call["id"] = id
				}
				if fragment, ok := tool["function"].(map[string]any); ok {
					for _, key := range []string{"name", "arguments"} {
						if text, ok := fragment[key].(string); ok {
							function := call["function"].(map[string]any)
							function[key] = stringField(function, key) + text
						}
					}
				}
			}
		}
	}
	for index := 0; index < len(calls); index++ {
		message["tool_calls"] = appendAny(message["tool_calls"], calls[index])
	}
	return message
}

// appendAny 为 Chat 测试聚合工具数组；参数为可选原数组及新项，返回数组，不修改客户端或外部状态。
func appendAny(value, item any) []any { items, _ := value.([]any); return append(items, item) }

// TestChatEncryptedReasoningToolRoundTrip 验证真实网关对 Responses 密文的 Chat JSON/SSE 输出、工具续轮和记账。
// 参数 t 为上下文；专属 PostgreSQL schema、本地上游、个人虚拟 Key。覆盖扩展回传/普通客户端忽略、空摘要、错误身份及连接变化，harness 完整清理。
func TestChatEncryptedReasoningToolRoundTrip(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, summary := range []bool{false, true} {
			for _, echo := range []bool{false, true} {
				t.Run(fmt.Sprintf("stream=%v/summary=%v/echo=%v", stream, summary, echo), func(t *testing.T) {
					var count atomic.Int32
					upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						var body map[string]any
						if r.URL.Path != "/v1/responses" || json.NewDecoder(r.Body).Decode(&body) != nil {
							t.Error("Chat 未使用登记的 Responses 上游")
							w.WriteHeader(400)
							return
						}
						count.Add(1)
						second := strings.Contains(string(mustJSON(body)), "file-value")
						if second && echo {
							found := false
							for _, value := range body["input"].([]any) {
								item := value.(map[string]any)
								if item["type"] == "reasoning" && item["encrypted_content"] == "test-cipher" && item["id"] == "rs_resp_1" {
									found = true
								}
							}
							if !found {
								t.Error("Chat 扩展未还原原始推理 ID 和密文")
							}
						}
						var call map[string]any
						if !second {
							call = map[string]any{"type": "function_call", "call_id": "call_1", "name": "read", "arguments": "{}"}
						}
						response := nativeAgentResponse(fmt.Sprintf("resp_%d", count.Load()), "Done", true, call)
						if !summary {
							response["output"].([]any)[0].(map[string]any)["summary"] = []any{}
						}
						writeAgentResponse(w, response, body["stream"] == true)
					}))
					t.Cleanup(upstream.Close)
					dep := chatDeployment("chat-encrypted-roundtrip")
					dep.LiteLLMParams["api_base"] = upstream.URL + "/v1"
					dep.ModelInfo["transport"] = "bypass_openai_responses"
					dep.ModelInfo["endpoint_types"] = []string{"chat"}
					dep.ModelInfo["pricing_source"] = "manual"
					h := newHarness(t)
					admin := h.adminSession()
					saved := h.ok("POST", "/model/new", admin, map[string]any{"model_name": dep.ModelName, "litellm_params": dep.LiteLLMParams, "model_info": dep.ModelInfo}).json()
					modelID := saved["model_info"].(map[string]any)["id"].(string)
					owner := h.openScope(t, admin, "chat-encrypted")
					headers := map[string]string{"X-Session-Id": "chat-session"}
					body := map[string]any{"model": dep.ModelName, "stream": stream, "messages": []any{map[string]any{"role": "user", "content": "Read the file."}}, "tools": []any{map[string]any{"type": "function", "function": map[string]any{"name": "read", "parameters": map[string]any{"type": "object"}}}}}
					first := h.doHeaders("POST", "/v1/chat/completions", owner.key, body, headers)
					if first.status != 200 || (stream && !strings.Contains(first.text(), "[DONE]")) {
						t.Fatal(first.describe())
					}
					message := chatReasoningMessage(t, first, stream)
					if len(message["reasoning_items"].([]any)) != 1 || len(message["tool_calls"].([]any)) != 1 {
						t.Fatalf("Chat 推理或工具丢失: %s", first.describe())
					}
					if !echo {
						delete(message, "reasoning_items")
					}
					body["messages"] = []any{map[string]any{"role": "user", "content": "Read the file."}, message, map[string]any{"role": "tool", "tool_call_id": "call_1", "content": "file-value"}}
					second := h.doHeaders("POST", "/v1/chat/completions", owner.key, body, headers)
					if second.status != 200 || chatReasoningMessage(t, second, stream)["content"] != "Done" || (stream && !strings.Contains(second.text(), "[DONE]")) {
						t.Fatalf("Chat 工具续轮失败: %s", second.describe())
					}
					before := count.Load()
					if invalid := h.doHeaders("POST", "/v1/chat/completions", "sk-invalid", body, headers); invalid.status != 401 {
						t.Fatal("非法 Key 未拒绝")
					}
					if echo {
						if invalid := h.doHeaders("POST", "/v1/chat/completions", owner.key, body, map[string]string{"X-Session-Id": "unowned"}); invalid.status != 400 {
							t.Fatalf("未归属 Chat 扩展被接受: %s", invalid.describe())
						}
						other := h.openScope(t, admin, "other-chat-owner")
						if invalid := h.doHeaders("POST", "/v1/chat/completions", other.key, body, headers); invalid.status != 400 {
							t.Fatalf("Chat 扩展跨身份重放: %s", invalid.describe())
						}
					}
					if count.Load() != before {
						t.Fatal("无效身份或扩展仍访问上游")
					}
					h.flushSpend()
					for _, response := range []reply{first, second} {
						bill := h.ok("GET", "/spend/logs/ui/"+response.header("x-litellm-call-id"), admin, nil).json()
						cost, _ := floatField(bill, "spend")
						if bill["status"] != "success" || bill["api_key"] != owner.keyID || !nearlyEqual(cost, 8*testInputRate+2*testOutputRate) {
							t.Fatalf("Chat 密文转换日志或记账错误: %v", bill)
						}
					}
					if echo {
						dep.LiteLLMParams["api_key"] = "sk-changed-account"
						h.ok("PATCH", "/model/"+modelID+"/update", admin, map[string]any{"litellm_params": dep.LiteLLMParams})
						if invalid := h.doHeaders("POST", "/v1/chat/completions", owner.key, body, headers); invalid.status != 409 || count.Load() != before {
							t.Fatalf("Chat 扩展被发送到变更后的连接: %s", invalid.describe())
						}
					}
				})
			}
		}
	}
}
