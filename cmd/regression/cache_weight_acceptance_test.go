package regression

import (
	"net/http"
	"strings"
	"testing"
)

// TestCacheWeightAcceptance 验证空回退配置下的缓存、密钥隔离、封禁和权重切换。
// 前置隔离数据库、真实网关和本地上游；验证独立日志、零费用和更新失效，
// 结束删除临时密钥、模板与部署，harness 回收 schema，无外部凭据。
func TestCacheWeightAcceptance(t *testing.T) {
	h := newHarness(t)
	admin := h.adminSession()
	owner := h.openScope(t, admin, "cache-weight")
	const model = "cache-weight-shared"
	ids := []string{"cache-weight-a", "cache-weight-b"}
	for i, id := range ids {
		h.addDBModel(t, admin, model, []string{"cache-a", "cache-b"}[i], id, nil)
	}
	// document 根据参数权重构造互斥分配及三类空回退，返回独立正文，供创建和更新接口使用，无副作用。
	document := func(a, b int) map[string]any {
		body := routeTemplateBody([]any{map[string]any{"model": model, "strategy": "traffic-split", "allocations": []any{map[string]any{"deployment_id": ids[0], "weight": a}, map[string]any{"deployment_id": ids[1], "weight": b}}}}, 1, 60, 0, 60)
		for _, field := range []string{"fallbacks", "context_window_fallbacks", "content_policy_fallbacks"} {
			body[field] = []any{map[string]any{model: []any{}}}
		}
		return body
	}
	template := routeTemplate(t, h, admin, "cache weight acceptance", document(100, 0))
	h.ok("POST", "/key/update", admin, map[string]any{"key": owner.key, "models": []string{model}, "route_template_id": template})
	isolated := stringField(h.ok("POST", "/key/generate", admin, map[string]any{"user_id": owner.userID, "team_id": owner.teamID, "models": []string{model}, "route_template_id": template}).json(), "key")
	defer func() {
		h.ok("POST", "/key/delete", admin, map[string]any{"keys": []string{owner.key, isolated}})
		h.ok("POST", "/route_template/"+template+"/delete", admin, map[string]any{})
		for _, id := range ids {
			h.ok("POST", "/model/delete", admin, map[string]any{"id": id})
		}
	}()
	request := chatRequest(model, "Reply only OK. cache weight acceptance")
	request["session_id"] = "cache-weight-session"
	first := h.ok("POST", "/v1/chat/completions", owner.key, request)
	mark := len(h.upstreamCalls())
	second := h.ok("POST", "/v1/chat/completions", owner.key, request)
	if isCacheHit(first) || !isCacheHit(second) || len(h.upstreamSince(mark)) != 0 || string(first.body) != string(second.body) || first.header("x-litellm-call-id") == second.header("x-litellm-call-id") {
		t.Fatalf("空回退下重复请求应缓存且有独立调用ID: first=%s second=%s", first.describe(), second.describe())
	}
	mark = len(h.upstreamCalls())
	other := h.ok("POST", "/v1/chat/completions", isolated, request)
	if isCacheHit(other) || strings.Join(h.upstreamSince(mark), ",") != "cache-a" {
		t.Fatalf("跨密钥缓存隔离失败: %s", other.describe())
	}
	h.ok("POST", "/key/block", admin, map[string]any{"key": owner.key})
	mark = len(h.upstreamCalls())
	denied := h.do("POST", "/v1/chat/completions", owner.key, request)
	if denied.status != http.StatusUnauthorized || isCacheHit(denied) || len(h.upstreamSince(mark)) != 0 {
		t.Fatalf("封禁密钥应在缓存读取前拒绝: %s", denied.describe())
	}
	h.ok("POST", "/key/unblock", admin, map[string]any{"key": owner.key})
	h.ok("POST", "/route_template/"+template+"/update", admin, map[string]any{"body": document(0, 100)})
	mark = len(h.upstreamCalls())
	switched := h.ok("POST", "/v1/chat/completions", owner.key, request)
	if isCacheHit(switched) || strings.Join(h.upstreamSince(mark), ",") != "cache-b" {
		t.Fatalf("权重更新应失效旧缓存并调用B: %s calls=%v", switched.describe(), h.upstreamSince(mark))
	}
	mark = len(h.upstreamCalls())
	repeated := h.ok("POST", "/v1/chat/completions", owner.key, request)
	if !isCacheHit(repeated) || len(h.upstreamSince(mark)) != 0 || string(repeated.body) != string(switched.body) {
		t.Fatalf("新权重重复请求应命中新缓存: %s", repeated.describe())
	}
	// 移除模板分配后继承默认权重；同会话同正文仍必须因平台默认更新而失效。
	inherited := routeTemplateBody([]any{}, 1, 60, 0, 60)
	h.ok("POST", "/route_template/"+template+"/update", admin, map[string]any{"body": inherited})
	for index, want := range []string{"cache-a", "cache-b"} {
		allocations := []any{map[string]any{"deployment_id": ids[0], "weight": 1 - index}, map[string]any{"deployment_id": ids[1], "weight": index}}
		h.ok("PUT", "/model/default", admin, map[string]any{"model_name": model, "weights": map[string]any{"allocations": allocations}})
		mark = len(h.upstreamCalls())
		result := h.ok("POST", "/v1/chat/completions", owner.key, request)
		if isCacheHit(result) || strings.Join(h.upstreamSince(mark), ",") != want {
			t.Fatalf("默认权重更新应失效同正文缓存并选%s: %s calls=%v", want, result.describe(), h.upstreamSince(mark))
		}
		mark = len(h.upstreamCalls())
		result = h.ok("POST", "/v1/chat/completions", owner.key, request)
		if !isCacheHit(result) || len(h.upstreamSince(mark)) != 0 {
			t.Fatalf("默认权重下重复应缓存: %s", result.describe())
		}
	}
	h.flushSpend()
	hits := 0
	for _, row := range logsFor(h.spendLogs(t, admin), model) {
		if cacheHit(row) {
			hits++
			if spend, _ := floatField(row, "spend"); spend != 0 {
				t.Fatalf("缓存命中不得收费: %+v", row)
			}
		}
	}
	if hits != 4 {
		t.Fatalf("应持久化四条零费用缓存命中日志，实际%d", hits)
	}
}
