package regression

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// TestRoutingGroupLifecycle 验证模板保存、绑定与持久化、组名与成员选路、权限、回退、计费和引用保护。
// 前置隔离PostgreSQL及本地上游；defer先解除绑定并清模板和回退再清部署，harness销毁schema和身份数据。
func TestRoutingGroupLifecycle(t *testing.T) {
	h := newHarness(t)
	admin := h.adminSession()
	owner := h.openScope(t, admin, "routing-group")
	var mu sync.Mutex
	var calls []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		model, _ := body["model"].(string)
		mu.Lock()
		calls = append(calls, model)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(model, "fail-") {
			w.WriteHeader(500)
			writeJSON(w, map[string]any{"error": map[string]any{"code": "api_error"}})
			return
		}
		writeJSON(w, map[string]any{"id": "group-chat", "object": "chat.completion", "model": model, "choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": "group-ok"}, "finish_reason": "stop"}}, "usage": map[string]any{"prompt_tokens": 11, "completion_tokens": 5, "total_tokens": 16}})
	}))
	defer upstream.Close()
	for i, name := range []string{"member-a", "member-b", "backup"} {
		h.ok("POST", "/model/new", admin, map[string]any{"model_name": name, "litellm_params": map[string]any{"model": "good-" + name, "api_base": upstream.URL, "api_key": "sk-local", "deployment_id": "group-" + name, "custom_llm_provider": "custom", "input_cost_per_token": float64(3-i) * 0.000001, "output_cost_per_token": 0.000002}, "model_info": map[string]any{"id": "group-" + name, "transport": "bypass_openai_chat", "endpoint_types": []string{"chat", "bypass:openai-chat"}, "pricing_source": "manual"}})
	}
	body := routeTemplateBody([]any{}, 1, 60, 0, 0)
	group := map[string]any{"group_name": "chat-group", "models": []string{"member-a", "member-b"}, "routing_strategy": "cost-based-routing"}
	body["routing_groups"] = []any{group}
	templateID := routeTemplate(t, h, admin, "group routing", body)
	h.ok("POST", "/route_template/binding", admin, map[string]any{"scope": "key", "scope_id": owner.keyID, "route_template_id": templateID})
	defer func() {
		h.ok("POST", "/route_template/binding", admin, map[string]any{"scope": "key", "scope_id": owner.keyID, "route_template_id": ""})
		h.ok("POST", "/route_template/"+templateID+"/delete", admin, map[string]any{})
		h.ok("PUT", "/model/fallback", admin, map[string]any{"model_name": "member-b", "policy": map[string]any{}})
		for _, name := range []string{"member-a", "member-b", "backup"} {
			h.ok("POST", "/model/delete", admin, map[string]any{"id": "group-" + name})
		}
	}()
	for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
		if r := h.do(method, "/routing/groups", admin, group); r.status != 404 {
			t.Fatalf("废弃独立组接口仍可调用: %s", r.describe())
		}
	}
	for _, action := range []string{"delete", "update"} {
		if r := h.do("POST", "/model/"+action, admin, map[string]any{"id": "group-member-a", "model_name": "renamed"}); r.status != 409 {
			t.Fatalf("模板组成员引用保护失效: %s", r.describe())
		}
	}
	if r := h.do("POST", "/model/update", admin, map[string]any{"id": "group-backup", "model_name": "chat-group"}); r.status != 409 {
		t.Fatalf("模型遮蔽模板组: %s", r.describe())
	}
	for _, path := range []string{"/v1/chat/completions", "/bypass/openai/v1/chat/completions"} {
		mu.Lock()
		calls = nil
		mu.Unlock()
		before := h.moneyOf(t, owner)
		h.ok("POST", path, owner.key, chatRequest("chat-group", "cost-group"+path))
		mu.Lock()
		got := strings.Join(calls, ",")
		mu.Unlock()
		if got != "good-member-b" {
			t.Fatalf("成本组实际选路: %s", got)
		}
		h.flushSpend()
		if h.moneyOf(t, owner).key <= before.key {
			t.Fatal("组调用未按实际部署计费")
		}
		mu.Lock()
		calls = nil
		mu.Unlock()
		h.ok("POST", path, owner.key, chatRequest("member-a", "member-only"+path))
		mu.Lock()
		got = strings.Join(calls, ",")
		mu.Unlock()
		if got != "good-member-a" {
			t.Fatalf("直接成员跨模型: %s", got)
		}
	}
	group["routing_strategy"] = "traffic-split"
	group["routing_strategy_args"] = map[string]any{"allocations": []any{map[string]any{"deployment_id": "group-member-a", "weight": 0}, map[string]any{"deployment_id": "group-member-b", "weight": 1}}}
	h.ok("POST", "/route_template/"+templateID+"/update", admin, map[string]any{"body": body})
	h.ok("POST", "/key/update", admin, map[string]any{"key": owner.key, "models": []string{"chat-group"}})
	mu.Lock()
	calls = nil
	mu.Unlock()
	if r := h.do("POST", "/v1/chat/completions", owner.key, chatRequest("chat-group", "denied-member")); r.status != 401 {
		t.Fatalf("成员权限绕过: %s", r.describe())
	}
	mu.Lock()
	n := len(calls)
	mu.Unlock()
	if n != 0 {
		t.Fatal("拒绝成员仍调用上游")
	}
	h.ok("POST", "/key/update", admin, map[string]any{"key": owner.key, "models": []string{}})
	h.ok("PUT", "/model/fallback", admin, map[string]any{"model_name": "member-b", "policy": map[string]any{"fallbacks": []string{"backup"}}})
	h.ok("POST", "/model/update", admin, map[string]any{"id": "group-member-b", "litellm_params": map[string]any{"model": "fail-member-b"}})
	for _, path := range []string{"/v1/chat/completions", "/bypass/openai/v1/chat/completions"} {
		mu.Lock()
		calls = nil
		mu.Unlock()
		h.ok("POST", path, owner.key, chatRequest("chat-group", "group-fallback"+path))
		mu.Lock()
		got := strings.Join(calls, ",")
		mu.Unlock()
		if got != "fail-member-b,good-backup" {
			t.Fatalf("组成员回退链: %s", got)
		}
	}
}
