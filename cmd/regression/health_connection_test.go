package regression

import (
	"net/http"
	"testing"

	"github.com/sunqirui1987/xhub/internal/config"
)

// TestHealthTestConnectionContract 通过真实网关路由验证连接测试的管理员鉴权、服务端部署选择、脱敏失败和零计费契约。
// 前置隔离 schema 和本地假上游；客户端伪造的地址、凭据与模型不得覆盖保存值，结束时 harness 清理服务与数据库，不调用付费服务。
func TestHealthTestConnectionContract(t *testing.T) {
	chat := chatDeployment("health-regression")
	chat.ModelInfo["id"] = "health-chat-deployment"
	media := config.ModelEntry{ModelName: "health-image", LiteLLMParams: map[string]any{
		"model": "image-upstream", "api_key": "local-secret",
	}, ModelInfo: map[string]any{"id": "health-image-deployment", "transport": "openai_image_generation", "endpoint_types": []string{"image_generation"}}}
	h := newHarness(t, chat, media)
	admin := h.adminSession()
	tenant := h.newTenant(t, admin, "health-member")
	member := h.signIn(t, tenant)
	request := map[string]any{
		"model":      "client-forged-model",
		"model_info": map[string]any{"id": "health-chat-deployment"},
		"mode":       "bypass_openai_chat",
		"litellm_params": map[string]any{
			"model": "client-forged-upstream", "api_base": "http://127.0.0.1:1/forged", "api_key": "client-forged-secret",
		},
	}

	if denied := h.do(http.MethodPost, "/health/test_connection", "", request); denied.status != http.StatusUnauthorized {
		t.Fatalf("未登录调用连接测试未被拒绝: %s", denied.describe())
	}
	if denied := h.do(http.MethodPost, "/health/test_connection", member, request); denied.status != http.StatusForbidden {
		t.Fatalf("普通成员调用连接测试未被拒绝: %s", denied.describe())
	}
	unknown := h.do(http.MethodPost, "/health/test_connection", admin, map[string]any{
		"model_info": map[string]any{"id": "missing-health-deployment"}, "mode": "bypass_openai_chat",
	})
	if unknown.status != http.StatusNotFound || len(h.upstreamCalls()) != 0 {
		t.Fatalf("未知部署未返回 404 或错误访问上游: %s calls=%v", unknown.describe(), h.upstreamCalls())
	}

	success := h.ok(http.MethodPost, "/health/test_connection", admin, request).json()
	if success["status"] != "success" || success["healthy"] != true || len(h.upstreamCalls()) != 1 {
		t.Fatalf("真实连接探测未访问保存部署: %v calls=%v", success, h.upstreamCalls())
	}
	call := h.upstreamCalls()[0]
	if call.Path != "/v1/chat/completions" || call.Body["model"] != "health-regression" {
		t.Fatalf("客户端参数覆盖了保存部署: path=%q body=%v", call.Path, call.Body)
	}

	h.scriptStatus("health-regression", http.StatusUnauthorized)
	failed := h.ok(http.MethodPost, "/health/test_connection", admin, map[string]any{
		"model_info": map[string]any{"id": "health-chat-deployment"}, "mode": "bypass_openai_chat",
	}).json()
	result, _ := failed["result"].(map[string]any)
	if failed["status"] != "error" || failed["healthy"] != false || failed["message"] == "" || result["error"] != failed["message"] {
		t.Fatalf("连接失败缺少页面可观察诊断: %v", failed)
	}

	before := len(h.upstreamCalls())
	unsupported := h.ok(http.MethodPost, "/health/test_connection", admin, map[string]any{
		"model_info": map[string]any{"id": "health-image-deployment"}, "mode": "openai_image_generation",
	}).json()
	if unsupported["error"] != "unsupported_mode" || len(h.upstreamCalls()) != before {
		t.Fatalf("媒体连接测试未明确拒绝或错误访问上游: %v calls=%v", unsupported, h.upstreamCalls())
	}
	if rows := logsFor(h.spendLogs(t, admin), "health-regression"); len(rows) != 0 {
		t.Fatalf("连接探测错误进入计费日志: %v", rows)
	}
}
