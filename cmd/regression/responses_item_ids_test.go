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

// completedItemResponse 提取真实 JSON/SSE 的 Responses 完成对象；参数为测试上下文、HTTP 回复及流模式，缺失终态则失败，无外部清理。
func completedItemResponse(t *testing.T, r reply, stream bool) map[string]any {
	t.Helper()
	if !stream {
		return r.json()
	}
	for _, line := range strings.Split(r.text(), "\n") {
		if strings.HasPrefix(line, "data: {") {
			var event map[string]any
			if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event) == nil && event["type"] == "response.completed" {
				return event["response"].(map[string]any)
			}
		}
	}
	t.Fatalf("Responses SSE 缺少完成对象: %s", r.describe())
	return nil
}

// TestResponsesItemIDModelSwitch 验证真实 HTTP/数据库的 Chat→Responses→严格原生模型切换，含旧历史、JSON/SSE、错误和记账。
// 前置两个本地上游、专属 PostgreSQL schema 和个人虚拟 Key；正常历史应放行，非法 ID/未认证仍拒绝；harness 回收身份、账单及服务。
func TestResponsesItemIDModelSwitch(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			var nativeCalls atomic.Int32
			chatUp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				encoded, _ := json.Marshal(body)
				if strings.Contains(string(encoded), "internal_chat_message_metadata_passthrough") || strings.Contains(string(encoded), `"phase"`) {
					w.WriteHeader(400)
					writeJSON(w, map[string]any{"error": map[string]any{"message": "Tracking or phase was not projected to Chat"}})
					return
				}
				if body["tools"] != nil {
					writeChatAgentResponse(w, "chatcmpl-source", "", "lookup_value", "{}", body["stream"] == true)
					return
				}
				writeChatAgentResponse(w, "chatcmpl-source", "Old answer", "", "", body["stream"] == true)
			}))
			t.Cleanup(chatUp.Close)
			nativeUp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nativeCalls.Add(1)
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				for _, value := range body["input"].([]any) {
					item := value.(map[string]any)
					id, _ := item["id"].(string)
					if item["internal_chat_message_metadata_passthrough"] != nil || item["metadata"] != nil {
						w.WriteHeader(400)
						writeJSON(w, map[string]any{"error": map[string]any{"message": "Unknown input message tracking metadata"}})
						return
					}
					prefix := ""
					if item["type"] == "message" {
						prefix = "msg_"
					}
					if item["type"] == "reasoning" {
						prefix = "rs_"
					}
					if item["type"] == "function_call" {
						prefix = "fc_"
						if item["call_id"] != "call_chat" {
							t.Error("函数历史关联 call_id 被改写")
						}
					}
					// 合成公开推理并非原厂存储对象：即使 rs_ 前缀合法，也不能携带这个引用 ID。
					if item["type"] == "reasoning" && (strings.HasPrefix(id, "rs_xhub_") || strings.Contains(id, "92ae29c5-") || item["status"] != nil) {
						w.WriteHeader(400)
						writeJSON(w, map[string]any{"error": map[string]any{"message": "Synthetic reasoning item not found"}})
						return
					}
					if id != "" && prefix != "" && !strings.HasPrefix(id, prefix) {
						w.WriteHeader(400)
						writeJSON(w, map[string]any{"error": map[string]any{"message": "Invalid item id prefix"}})
						return
					}
				}
				writeAgentResponse(w, nativeAgentResponse("resp_strict", "Switched OK", false, nil), body["stream"] == true)
			}))
			t.Cleanup(nativeUp.Close)
			chat, native := chatDeployment("bridge-source"), chatDeployment("strict-native")
			chat.LiteLLMParams["api_base"] = chatUp.URL + "/v1"
			native.LiteLLMParams["api_base"] = nativeUp.URL + "/v1"
			chat.ModelInfo["transport"] = "bypass_openai_chat"
			native.ModelInfo["transport"] = "bypass_openai_responses"
			chat.ModelInfo["endpoint_types"], native.ModelInfo["endpoint_types"] = []string{"responses"}, []string{"responses"}
			h := newHarness(t, chat, native)
			admin := h.adminSession()
			owner := h.openScope(t, admin, "item-switch")
			first := h.ok("POST", "/v1/responses", owner.key, map[string]any{"model": chat.ModelName, "input": "Hello", "stream": stream})
			items := completedItemResponse(t, first, stream)["output"].([]any)
			for _, value := range items {
				item := value.(map[string]any)
				prefix := "msg_"
				if item["type"] == "reasoning" {
					prefix = "rs_"
				}
				if !strings.HasPrefix(item["id"].(string), prefix) {
					t.Fatalf("Chat 桥接生成了错误 item ID: %v", item)
				}
			}
			input := append([]any{map[string]any{"role": "user", "content": "Hello"}}, items...)
			input = append(input, map[string]any{"role": "user", "content": "Continue"})
			second := h.ok("POST", "/v1/responses", owner.key, map[string]any{"model": native.ModelName, "input": input, "stream": stream, "store": false})
			output := completedItemResponse(t, second, stream)["output"].([]any)
			if output[0].(map[string]any)["id"] != "msg_resp_strict" {
				t.Fatal("合法原生 item ID 被改写")
			}
			legacy := []any{map[string]any{"role": "user", "content": "Hello"}, map[string]any{"type": "reasoning", "id": "chatcmpl-old_reasoning", "summary": []any{map[string]any{"type": "summary_text", "text": "Plan"}}}, map[string]any{"type": "message", "id": "chatcmpl-old_message", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": "Old answer"}}}, map[string]any{"role": "user", "content": "Continue"}}
			h.ok("POST", "/v1/responses", owner.key, map[string]any{"model": native.ModelName, "input": legacy, "stream": stream})
			// Grok 的 Chat 响应级 UUID 与原生公开推理 UUID/status 都不能作为 OpenAI 存储引用回放。
			for _, nativeGrok := range []bool{false, true} {
				const seed = "92ae29c5-9e2c-9cbd-a94b-2fd2abc135be"
				grokReasoning := map[string]any{"type": "reasoning", "id": seed + "_reasoning", "summary": []any{map[string]any{"type": "summary_text", "text": "Public plan"}}}
				grokMessage := map[string]any{"type": "message", "id": seed + "_message", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": "Old Grok answer"}}}
				if nativeGrok {
					grokReasoning["id"], grokReasoning["status"], grokMessage["id"] = "rs_"+seed, "completed", "msg_"+seed
				}
				grok := []any{grokReasoning, grokMessage, map[string]any{"role": "user", "content": "Continue"}}
				r := h.ok("POST", "/v1/responses", owner.key, map[string]any{"model": native.ModelName, "input": grok, "stream": stream})
				if completedItemResponse(t, r, stream)["status"] != "completed" {
					t.Fatalf("Grok 完整公开历史跨模型回放失败: %s", r.describe())
				}
			}
			// Fenno 偶发原样输出内部追踪字段：原生回放清理跟踪字段，Chat 投影复用既有 phase 映射，不丢原文。
			for _, target := range []string{native.ModelName, chat.ModelName} {
				tracked := []any{map[string]any{"type": "message", "id": "msg_tracked", "role": "assistant", "phase": "final_answer", "content": []any{map[string]any{"type": "output_text", "text": "Old answer"}}, "internal_chat_message_metadata_passthrough": map[string]any{"create_time": 1.0, "turn_id": "trace"}, "metadata": map[string]any{"turn_id": "trace"}}, map[string]any{"role": "user", "content": "Continue"}}
				r := h.ok("POST", "/v1/responses", owner.key, map[string]any{"model": target, "input": tracked, "stream": stream})
				if completedItemResponse(t, r, stream)["status"] != "completed" {
					t.Fatalf("带供应商输出追踪字段的历史回放失败: %s", r.describe())
				}
			}
			legacy[2].(map[string]any)["id"] = "foreign_invalid"
			// 真实 Chat 工具产物必须分离函数 item.id 与 call_id；两种历史形状都能带结果切换原生 GPT。
			tool := []any{map[string]any{"type": "function", "name": "lookup_value", "parameters": map[string]any{"type": "object"}}}
			toolReply := h.ok("POST", "/v1/responses", owner.key, map[string]any{"model": chat.ModelName, "input": "Call lookup_value", "tools": tool, "stream": stream})
			toolItems := completedItemResponse(t, toolReply, stream)["output"].([]any)
			for _, oldTool := range []bool{false, true} {
				for _, value := range toolItems {
					item := value.(map[string]any)
					if item["type"] == "function_call" {
						if !oldTool && !strings.HasPrefix(item["id"].(string), "fc_") {
							t.Fatalf("新函数历史仍使用关联 ID: %v", item)
						}
						if oldTool {
							item["id"] = item["call_id"]
						}
					}
				}
				input := append([]any{}, toolItems...)
				input = append(input, map[string]any{"type": "function_call_output", "call_id": "call_chat", "output": "KNOWN_TEST_VALUE"})
				h.ok("POST", "/v1/responses", owner.key, map[string]any{"model": native.ModelName, "input": input, "stream": stream})
			}
			bad := h.do("POST", "/v1/responses", owner.key, map[string]any{"model": native.ModelName, "input": legacy})
			if bad.status != 400 || !strings.Contains(bad.text(), "Invalid item id prefix") {
				t.Fatalf("任意错误 ID 被静默改写: %s", bad.describe())
			}
			before := nativeCalls.Load()
			unauth := h.do("POST", "/v1/responses", "", map[string]any{"model": native.ModelName, "input": "Hello"})
			if unauth.status != 401 || nativeCalls.Load() != before {
				t.Fatalf("缺少凭据的请求触达上游: %s", unauth.describe())
			}
			h.flushSpend()
			for _, r := range []reply{first, second} {
				bill := h.ok("GET", "/spend/logs/ui/"+r.header("x-litellm-call-id"), admin, nil).json()
				if bill["status"] != "success" || bill["api_key"] != owner.keyID {
					t.Fatalf("跨模型续聊记账归属错误: %v", bill)
				}
			}
		})
	}
}
