package regression

import (
	"net/http"
	"testing"
)

// TestGroupRequestConsumesOneRPM 验证一次组请求只占一次调用方 RPM。参数 t 为测试上下文，返回无。
// 前置真实网关、隔离 PostgreSQL 和本地上游；首个请求应成功，第二个应限流；harness 清理 schema 和服务。
func TestGroupRequestConsumesOneRPM(t *testing.T) {
	h := newHarness(t, chatDeployment("audit-member"))
	admin := h.adminSession()
	owner := h.openScope(t, admin, "audit-rate")
	body := routeTemplateBody([]any{}, 1, 60, 0, 0)
	body["routing_groups"] = []any{map[string]any{"group_name": "audit-group", "models": []string{"audit-member"}, "routing_strategy": "random"}}
	id := routeTemplate(t, h, admin, "audit group rpm", body)
	bindTemplate(t, h, admin, "key", owner.keyID, id)
	h.ok("POST", "/key/update", admin, map[string]any{"key": owner.key, "rpm_limit": 1})
	r := h.do("POST", "/v1/chat/completions", owner.key, chatRequest("audit-group", "Reply only OK."))
	if r.status != http.StatusOK || len(h.upstreamCalls()) != 1 {
		t.Fatalf("RPM=1 的首个组请求应成功并只调一次上游，实际 status=%d upstream=%d response=%s", r.status, len(h.upstreamCalls()), r.describe())
	}
	second := h.do("POST", "/v1/chat/completions", owner.key, chatRequest("audit-group", "Reply only NEXT."))
	if second.status != http.StatusTooManyRequests || len(h.upstreamCalls()) != 1 {
		t.Fatalf("第二个组请求应限流且不调用上游，实际 %s", second.describe())
	}
}

// TestRetiredRouterSettings 验证旧全局设置接口被移除且混合写入原子拒绝。
// 前置真实网关、隔离数据库和本地上游；失败不能留下 general_settings 变更；harness 清理全部数据。
func TestRetiredRouterSettings(t *testing.T) {
	h := newHarness(t, chatDeployment("template-only"))
	admin := h.adminSession()
	for _, path := range []string{"/router/settings", "/router/fields"} {
		got := h.do("GET", path, admin, nil)
		if got.status != 404 {
			t.Fatalf("废弃路由仍暴露: %s %s", path, got.describe())
		}
	}
	before := h.ok("GET", "/config/list", admin, nil)
	got := h.do("POST", "/config/update", admin, map[string]any{"router_settings": map[string]any{"num_retries": 3}, "general_settings": map[string]any{"store_model_in_db": false}})
	if got.status != 400 {
		t.Fatalf("旧全局配置应拒绝: %s", got.describe())
	}
	after := h.ok("GET", "/config/list", admin, nil)
	if before.text() != after.text() {
		t.Fatalf("失败请求留下部分写入: before=%s after=%s", before.describe(), after.describe())
	}
}
