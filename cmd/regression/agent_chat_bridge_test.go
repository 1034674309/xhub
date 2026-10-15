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

// writeChatAgentResponse 输出带公开推理和后置 usage 的 Chat JSON/SSE 夹具。
// 参数为写入器、响应 ID、正文、可空函数名/参数及流开关；后台契约与真实 CLI 夹具调用，无返回，不执行工具，服务关闭即回收。
func writeChatAgentResponse(w http.ResponseWriter, id, text, name, arguments string, stream bool) {
	message := map[string]any{"role": "assistant", "content": text, "reasoning_content": "Inspect the file before answering."}
	stop := "stop"
	if name != "" {
		message["content"] = nil
		message["tool_calls"] = []any{map[string]any{"index": 0, "id": "call_chat", "type": "function", "function": map[string]any{"name": name, "arguments": arguments}}}
		stop = "tool_calls"
	}
	usage := map[string]any{"prompt_tokens": 8, "completion_tokens": 2}
	if !stream {
		writeJSON(w, map[string]any{"id": id, "choices": []any{map[string]any{"message": message, "finish_reason": stop}}, "usage": usage})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	chunk, _ := json.Marshal(map[string]any{"id": id, "choices": []any{map[string]any{"index": 0, "delta": message, "finish_reason": stop}}})
	stats, _ := json.Marshal(map[string]any{"id": id, "choices": []any{}, "usage": usage})
	fmt.Fprintf(w, "data: %s\n\ndata: %s\n\ndata: [DONE]\n\n", chunk, stats)
}

// TestAgentChatBridgeTools 验证真实后台将 Codex 的 namespace/custom、推理提示、工具续轮映射到 Chat 并正确记账。
// 参数 t 为上下文；前置隔离 PostgreSQL、真实 HTTP 和本地上游，覆盖 JSON/SSE、非法输入、鉴权和调用归属；harness 清理所有身份、模型与账单。
func TestAgentChatBridgeTools(t *testing.T) {
	for _, custom := range []bool{false, true} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("custom=%v/stream=%v", custom, stream), func(t *testing.T) {
				var count atomic.Int32
				up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var body map[string]any
					_ = json.NewDecoder(r.Body).Decode(&body)
					count.Add(1)
					if r.URL.Path != "/v1/chat/completions" || body["include"] != nil || body["client_metadata"] != nil || body["reasoning_effort"] != "medium" {
						t.Error("Chat 桥接路径或请求提示处理错误")
						w.WriteHeader(400)
						return
					}
					raw := string(mustJSON(body["messages"]))
					if strings.Contains(raw, "file-value") {
						if !strings.Contains(raw, "Inspect the file before answering.") || !strings.Contains(raw, "call_chat") || !strings.Contains(raw, "fs__read") {
							t.Error("Chat 工具续轮丢失推理或调用身份")
						}
						writeChatAgentResponse(w, "r_chat_done", "AGENT_OK file-value", "", "", stream)
						return
					}
					tools, _ := body["tools"].([]any)
					if len(tools) != 1 || tools[0].(map[string]any)["function"].(map[string]any)["name"] != "fs__read" {
						t.Error("namespace 未映射为 Chat 函数")
						w.WriteHeader(400)
						return
					}
					args := `{}`
					if custom {
						args = `{"content":"Read the file."}`
					}
					writeChatAgentResponse(w, "r_chat_tool", "", "fs__read", args, stream)
				}))
				t.Cleanup(up.Close)
				dep := chatDeployment("agent-chat-bridge")
				dep.LiteLLMParams["api_base"] = up.URL + "/v1"
				dep.ModelInfo["transport"] = "bypass_openai_chat"
				dep.ModelInfo["endpoint_types"] = []string{"responses"}
				h := newHarness(t, dep)
				admin := h.adminSession()
				owner := h.openScope(t, admin, "chat-agent")
				tool := map[string]any{"type": "function", "name": "read", "parameters": map[string]any{"type": "object"}}
				if custom {
					tool = map[string]any{"type": "custom", "name": "read"}
				}
				body := map[string]any{"model": dep.ModelName, "input": "Read the file.", "stream": stream,
					"reasoning": map[string]any{"effort": "medium", "summary": "auto", "context": "all_turns"}, "include": []string{"reasoning.encrypted_content"}, "client_metadata": map[string]any{"session_id": "client-only"},
					"tools": []any{map[string]any{"type": "namespace", "name": "fs", "tools": []any{tool}}}}
				if custom {
					body["input"] = []any{map[string]any{"type": "additional_tools", "tools": body["tools"]}, map[string]any{"role": "user", "content": "Read the file."}}
					delete(body, "tools")
				}
				first := h.ok("POST", "/v1/responses", owner.key, body)
				if !strings.Contains(first.text(), `"namespace":"fs"`) || !strings.Contains(first.text(), `"name":"read"`) {
					t.Fatalf("客户端工具身份没有恢复: %s", first.describe())
				}
				kind := "function_call"
				call := map[string]any{"type": kind, "namespace": "fs", "name": "read", "call_id": "call_chat", "arguments": "{}"}
				if custom {
					kind = "custom_tool_call"
					call["type"], call["input"] = kind, "Read the file."
					delete(call, "arguments")
				}
				if !strings.Contains(first.text(), `"type":"`+kind+`"`) {
					t.Fatalf("工具类型未恢复: %s", first.describe())
				}
				body["input"] = []any{map[string]any{"role": "user", "content": "Read the file."}, map[string]any{"type": "reasoning", "id": "rs_public", "summary": []any{map[string]any{"type": "summary_text", "text": "Inspect the file before answering."}}}, call, map[string]any{"type": kind + "_output", "call_id": "call_chat", "output": "file-value"}}
				if custom {
					body["input"] = append([]any{map[string]any{"type": "additional_tools", "tools": []any{map[string]any{"type": "namespace", "name": "fs", "tools": []any{tool}}}}}, body["input"].([]any)...)
				}
				second := h.ok("POST", "/v1/responses", owner.key, body)
				if !strings.Contains(second.text(), "AGENT_OK file-value") {
					t.Fatalf("工具结果未完成后续回答: %s", second.describe())
				}
				h.flushSpend()
				for _, response := range []reply{first, second} {
					bill := h.ok("GET", "/spend/logs/ui/"+response.header("x-litellm-call-id"), admin, nil).json()
					amount, _ := floatField(bill, "spend")
					if bill["status"] != "success" || bill["api_key"] != owner.keyID || !nearlyEqual(amount, 8*testInputRate+2*testOutputRate) {
						t.Fatalf("工具桥接账单或身份错误: %v", bill)
					}
				}
				before := count.Load()
				body["include"] = []string{"unknown_native_capability"}
				if bad := h.do("POST", "/v1/responses", owner.key, body); bad.status != 400 {
					t.Fatalf("未知能力没有在选路时拒绝: %s", bad.describe())
				}
				if bad := h.do("POST", "/v1/responses", "sk-invalid", body); bad.status != 401 {
					t.Fatalf("错误密钥未拒绝: %s", bad.describe())
				}
				body["include"], body["input"] = []any{}, "Hello"
				body["tools"] = []any{map[string]any{"type": "function", "name": "fs__read", "parameters": map[string]any{}}, map[string]any{"type": "namespace", "name": "fs", "tools": []any{map[string]any{"type": "function", "name": "read", "parameters": map[string]any{}}}}}
				if bad := h.do("POST", "/v1/responses", owner.key, body); bad.status != 400 {
					t.Fatalf("Chat 别名冲突未在外发前拒绝: %s", bad.describe())
				}
				body["tools"] = []any{map[string]any{"type": "custom", "name": "run", "format": map[string]any{"type": "unknown_native_format"}}}
				if bad := h.do("POST", "/v1/responses", owner.key, body); bad.status != 400 {
					t.Fatalf("未知 custom 格式未在外发前拒绝: %s", bad.describe())
				}
				if count.Load() != before {
					t.Fatal("失败输入或鉴权请求仍被发送到上游")
				}
			})
		}
	}
}
