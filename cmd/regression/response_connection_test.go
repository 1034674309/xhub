package regression

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// TestResponseConnectionChanges 验证保留部署 ID 修改连接后，旧父响应和完整密文均在外发前被拒绝。
// 参数 t 为上下文；真实 HTTP/PostgreSQL 创建供应商、用户、模型及账单，覆盖统一/原生入口和地址、型号、凭据名、同名凭据更新。
// 验证原连接续接、未授权拒绝、普通请求仍可重新选路、成功记账；本地 HTTP 上游和私有 schema 由 Cleanup/harness 回收。
func TestResponseConnectionChanges(t *testing.T) {
	for _, path := range []string{"/v1/responses", "/bypass/openai/v1/responses"} {
		for _, mutation := range []string{"api_base", "model", "credential", "credential_values"} {
			t.Run(path+"/"+mutation, func(t *testing.T) {
				var calls atomic.Int32
				up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var body map[string]any
					_ = json.NewDecoder(r.Body).Decode(&body)
					n := calls.Add(1)
					writeAgentResponse(w, nativeAgentResponse(fmt.Sprintf("resp_connection_%d", n), "Done", true, nil), body["stream"] == true)
				}))
				t.Cleanup(up.Close)
				h := newHarness(t)
				admin := h.adminSession()
				owner := h.openScope(t, admin, "connection-change")
				for _, name := range []string{"connection-a", "connection-b"} {
					h.ok("POST", "/credentials", admin, map[string]any{"credential_name": name, "credential_info": map[string]any{"custom_llm_provider": "openai"}, "credential_values": map[string]any{"api_base": up.URL + "/v1", "api_key": "sk-" + name}})
				}
				params := map[string]any{"model": "original-upstream", "custom_llm_provider": "openai", "litellm_credential_name": "connection-a", "input_cost_per_token": testInputRate, "output_cost_per_token": testOutputRate}
				saved := h.ok("POST", "/model/new", admin, map[string]any{"model_name": "connection-change", "litellm_params": params, "model_info": map[string]any{"transport": "bypass_openai_responses", "pricing_source": "manual", "endpoint_types": []string{"responses", "bypass:openai-responses"}}}).json()
				id := saved["model_info"].(map[string]any)["id"].(string)
				headers := map[string]string{"X-Session-Id": "connection-change-session"}
				first := h.doHeaders("POST", path, owner.key, map[string]any{"model": "connection-change", "input": "Hello"}, headers)
				if first.status != 200 {
					t.Fatal(first.describe())
				}
				previous := map[string]any{"model": "connection-change", "previous_response_id": first.json()["id"], "input": "Continue"}
				valid := h.doHeaders("POST", path, owner.key, previous, headers)
				if valid.status != 200 || calls.Load() != 2 {
					t.Fatalf("原连接不能正常续接: %s", valid.describe())
				}
				full := map[string]any{"model": "connection-change", "input": append([]any{map[string]any{"role": "user", "content": "Hello"}}, first.json()["output"].([]any)...)}
				switch mutation {
				case "api_base":
					params["api_base"] = up.URL + "/changed/v1"
				case "model":
					params["model"] = "changed-upstream"
				case "credential":
					params["litellm_credential_name"] = "connection-b"
				case "credential_values":
					h.ok("PATCH", "/credentials/connection-a", admin, map[string]any{"credential_values": map[string]any{"api_base": up.URL + "/v1", "api_key": "sk-changed-account"}})
				}
				if mutation != "credential_values" {
					h.ok("PATCH", "/model/"+id+"/update", admin, map[string]any{"litellm_params": params})
				}
				for _, body := range []map[string]any{previous, full} {
					blocked := h.doHeaders("POST", path, owner.key, body, headers)
					if blocked.status != 409 || calls.Load() != 2 {
						t.Fatalf("连接变化后历史仍外发: %s %s", mutation, blocked.describe())
					}
				}
				if bad := h.doHeaders("POST", path, "sk-invalid", previous, headers); bad.status != 401 || calls.Load() != 2 {
					t.Fatalf("未授权续接没有拒绝: %s", bad.describe())
				}
				// 即便同一会话先成功执行普通请求，动态偏好也不能替旧密文重新签发连接归属。
				fresh := h.doHeaders("POST", path, owner.key, map[string]any{"model": "connection-change", "input": "A fresh plain request"}, headers)
				if fresh.status != 200 || calls.Load() != 3 {
					t.Fatalf("普通请求不能重新选路: %s", fresh.describe())
				}
				blocked := h.doHeaders("POST", path, owner.key, full, headers)
				if blocked.status != 409 || calls.Load() != 3 {
					t.Fatalf("普通偏好更新绕过了旧历史归属: %s", blocked.describe())
				}
				h.flushSpend()
				for _, response := range []reply{first, valid, fresh} {
					bill := h.ok("GET", "/spend/logs/ui/"+response.header("x-litellm-call-id"), admin, nil).json()
					if bill["status"] != "success" || bill["api_key"] != owner.keyID {
						t.Fatalf("连接检查影响记账归属: %v", bill)
					}
				}
			})
		}
	}
}
