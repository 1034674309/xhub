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

// TestWebSearchDeploymentPolicy 通过真实管理/推理/日志路由验证搜索策略保存、JSON/SSE 降级及强制搜索拒绝。
// 前置隔离 PostgreSQL 和本地 Chat 上游；断言原提示与普通工具保留、费用归属、诊断落库和非法配置 400。
// finally 删除自建模型，harness 清理账号、账单及专属 schema，不使用外部服务或真实密钥。
func TestWebSearchDeploymentPolicy(t *testing.T) {
	var count atomic.Int32
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if r.URL.Path != "/v1/chat/completions" || strings.Contains(string(mustJSON(body)), "web_search") {
			t.Error("搜索声明未从真实出站请求移除")
			w.WriteHeader(400)
			return
		}
		if !strings.Contains(string(mustJSON(body["messages"])), "Keep my original input.") {
			t.Error("降级改变用户提示")
		}
		tools, _ := body["tools"].([]any)
		if len(tools) != 1 || tools[0].(map[string]any)["function"].(map[string]any)["name"] != "read" {
			t.Error("普通函数被降级删除")
		}
		stream, _ := body["stream"].(bool)
		writeChatAgentResponse(w, "search_policy_ok", "Policy OK", "", "", stream)
	}))
	defer up.Close()
	h := newHarness(t)
	admin := h.adminSession()
	dep := chatDeployment("search-policy")
	dep.LiteLLMParams["api_base"] = up.URL + "/v1"
	dep.ModelInfo["endpoint_types"] = []string{"responses"}
	dep.ModelInfo["web_search_support"] = "unsupported"
	dep.ModelInfo["web_search_fallback"] = "drop_optional"
	saved := h.ok("POST", "/model/new", admin, map[string]any{"model_name": dep.ModelName, "litellm_params": dep.LiteLLMParams, "model_info": dep.ModelInfo}).json()
	id := saved["model_info"].(map[string]any)["id"]
	defer h.ok("POST", "/model/delete", admin, map[string]any{"id": id})
	owner := h.openScope(t, admin, "search-policy-owner")
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint("stream=", stream), func(t *testing.T) {
			body := map[string]any{"model": dep.ModelName, "input": "Keep my original input.", "stream": stream, "tools": []any{map[string]any{"type": "web_search"}, map[string]any{"type": "function", "name": "read", "parameters": map[string]any{"type": "object"}}}}
			res := h.ok("POST", "/v1/responses", owner.key, body)
			if res.header("X-XHub-Dropped-Capabilities") != "web_search" || !strings.Contains(res.text(), "Policy OK") {
				t.Fatalf("降级响应标记或结果错误: %s", res.describe())
			}
			h.flushSpend()
			detail := logDetail(t, h, admin, res.header("x-litellm-call-id"))
			meta := detail["metadata"].(map[string]any)
			diagnostic := meta["upstream_response"].(map[string]any)
			if string(mustJSON(diagnostic["dropped_capabilities"])) != `["web_search"]` || detail["api_key"] != owner.keyID || detail["status"] != "success" {
				t.Fatalf("降级诊断/记账未正确落库: %v", detail)
			}
			stored, err := h.db.GetRequestLog(t.Context(), res.header("x-litellm-call-id"))
			if err != nil || !strings.Contains(stored.UpstreamResponse, "dropped_capabilities") {
				t.Fatalf("降级诊断没有持久化: %v", err)
			}
			before := count.Load()
			for _, choice := range []any{"required", map[string]any{"type": "web_search"}} {
				body["tool_choice"] = choice
				bad := h.do("POST", "/v1/responses", owner.key, body)
				if bad.status != 400 || !strings.Contains(bad.text(), "web_search") || !strings.Contains(bad.text(), "unsupported_capability") {
					t.Fatalf("强制搜索错误不明确: %s", bad.describe())
				}
			}
			if count.Load() != before {
				t.Fatal("强制搜索拒绝后仍调用上游")
			}
		})
	}
	for _, badInfo := range []map[string]any{{"web_search_support": "bad"}, {"web_search_fallback": true}, {"web_search_support": "supported"}} {

		badInfo["id"] = id
		res := h.do("PATCH", fmt.Sprint("/model/", id, "/update"), admin, map[string]any{"model_info": badInfo})
		if res.status != 400 {
			t.Fatalf("非法搜索配置没有拒绝: %s", res.describe())
		}
	}
}
