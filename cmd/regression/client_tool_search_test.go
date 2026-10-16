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

// TestClientToolSearchFlow 验证真实网关的搜索降级、客户端工具发现、七牛 input 包装及工具结果三轮 JSON/SSE 调用。
// 参数 t 为上下文；私有数据库和本地 Chat 上游验证请求契约、失败边界、日志/归属/收费，harness 清理身份和 schema。
func TestClientToolSearchFlow(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint("stream=", stream), func(t *testing.T) {
			var count atomic.Int32
			up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count.Add(1)
				var body map[string]any
				if json.NewDecoder(r.Body).Decode(&body) != nil {
					t.Error("Chat 请求不是 JSON")
					w.WriteHeader(400)
					return
				}
				raw := string(mustJSON(body))
				if r.URL.Path != "/v1/chat/completions" || strings.Contains(raw, `"type":"web_search"`) || strings.Contains(raw, `"type":"tool_search"`) || strings.Contains(raw, `"defer_loading":true`) {
					t.Error("客户端发现/搜索声明未转换为 Chat 函数契约")
				}
				switch count.Load() {
				case 1:
					writeChatAgentResponse(w, "search", "", "tool_search", `{"query":"read a file"}`, stream, "search_call")
				case 2:
					if !strings.Contains(raw, "fs__read") || !strings.Contains(raw, `"tool_call_id":"search_call"`) {
						t.Error("搜索结果没有加载工具或丢失搜索调用 ID")
					}
					writeChatAgentResponse(w, "read", "", "fs__read", `{"input":"Read the file."}`, stream, "read_call")
				case 3:
					if !strings.Contains(raw, "file-value") {
						t.Error("工具执行结果没有传入最终续轮")
					}
					writeChatAgentResponse(w, "done", "AGENT_OK file-value", "", "", stream)
				default:
					t.Error("失败请求不应调用供应商")
					w.WriteHeader(400)
				}
			}))
			defer up.Close()
			dep := chatDeployment("client-search")
			dep.LiteLLMParams["api_base"] = up.URL + "/v1"
			dep.ModelInfo["web_search_support"], dep.ModelInfo["web_search_fallback"] = "unsupported", "drop_optional"
			h := newHarness(t, dep)
			admin := h.adminSession()
			owner := h.openScope(t, admin, "client-search")
			search := map[string]any{"type": "tool_search", "execution": "client", "description": "Discover tools", "parameters": map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}}}}
			body := map[string]any{"model": dep.ModelName, "input": "Read the file.", "stream": stream, "tools": []any{map[string]any{"type": "web_search"}, search}}
			first := h.ok("POST", "/v1/responses", owner.key, body)
			if !strings.Contains(first.text(), `"type":"tool_search_call"`) || !strings.Contains(first.text(), `"execution":"client"`) || !strings.Contains(first.text(), `"arguments":{"query":"read a file"}`) {
				t.Fatalf("搜索调用未恢复为客户端执行: %s", first.describe())
			}
			loaded := []any{map[string]any{"type": "namespace", "name": "fs", "tools": []any{map[string]any{"type": "custom", "name": "read", "defer_loading": true}}}}
			history := []any{map[string]any{"role": "user", "content": "Read the file."}, map[string]any{"type": "tool_search_call", "call_id": "search_call", "execution": "client", "arguments": map[string]any{"query": "read a file"}}, map[string]any{"type": "tool_search_output", "execution": "client", "call_id": "search_call", "tools": loaded}}
			body["input"] = history
			second := h.ok("POST", "/v1/responses", owner.key, body)
			if !strings.Contains(second.text(), `"type":"custom_tool_call"`) || !strings.Contains(second.text(), `"input":"Read the file."`) || !strings.Contains(second.text(), `"namespace":"fs"`) {
				t.Fatalf("七牛 custom input 别名未还原: %s", second.describe())
			}
			body["input"] = append(history, map[string]any{"type": "custom_tool_call", "name": "read", "namespace": "fs", "call_id": "read_call", "input": "Read the file."}, map[string]any{"type": "custom_tool_call_output", "call_id": "read_call", "output": "file-value"})
			third := h.ok("POST", "/v1/responses", owner.key, body)
			if !strings.Contains(third.text(), "AGENT_OK file-value") {
				t.Fatalf("工具发现后的最终回答未完成: %s", third.describe())
			}
			h.flushSpend()
			for _, response := range []reply{first, second, third} {
				bill := logDetail(t, h, admin, response.header("x-litellm-call-id"))
				if response.header("X-XHub-Dropped-Capabilities") != "web_search" || bill["status"] != "success" || bill["api_key"] != owner.keyID || !strings.Contains(string(mustJSON(bill)), "dropped_capabilities") {
					t.Fatalf("搜索降级的账单、身份或诊断未持久化: %v", bill)
				}
				amount, _ := floatField(bill, "spend")
				if !nearlyEqual(amount, 8*testInputRate+2*testOutputRate) {
					t.Fatalf("三轮工具调用计费错误: %v", bill)
				}
			}
			body["input"] = "Hello"
			search["execution"] = "server"
			if rejected := h.do("POST", "/v1/responses", owner.key, body); rejected.status != 400 {
				t.Fatalf("托管搜索不应伪装成客户端函数: %s", rejected.describe())
			}
			if count.Load() != 3 {
				t.Fatal("不支持的搜索仍调用了供应商")
			}
		})
	}
}
