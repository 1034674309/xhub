package regression

import (
	"net/http"
	"reflect"
	"testing"
)

// templateBody 返回模板详情中的正文对象；缺少正文时立即终止测试。
func templateBody(t *testing.T, h *harness, admin, id string) map[string]any {
	t.Helper()
	row := h.ok(http.MethodGet, "/route_template/"+id, admin, nil).json()
	body, _ := row["body"].(map[string]any)
	if body == nil {
		t.Fatalf("模板详情缺少 body: %v", row)
	}
	return body
}

// TestRouteTemplateConfigurationRoundTrip 验证完整新 JSON 原样保存和整体替换。
// 模板正文保留已声明的模型规则、可靠性、组及回退，不隐式补字段。
func TestRouteTemplateConfigurationRoundTrip(t *testing.T) {
	h := newHarness(t)
	admin := h.adminSession()
	first := routeTemplateBody([]any{
		map[string]any{"model": "gpt-6-sol", "strategy": "least-busy"},
		map[string]any{"model": "gpt-5.5", "strategy": "traffic-split"},
	}, 2, 45, 4, 20)
	id := routeTemplate(t, h, admin, "strict document", first)
	if got := templateBody(t, h, admin, id); !reflect.DeepEqual(got, first) {
		t.Fatalf("模板保存后发生隐式补全或改写: got=%v want=%v", got, first)
	}

	second := routeTemplateBody([]any{map[string]any{"model": "gpt-6-sol", "strategy": "random"}}, 1, 30, 0, 0)
	h.ok(http.MethodPost, "/route_template/"+id+"/update", admin, map[string]any{"body": second})
	if got := templateBody(t, h, admin, id); !reflect.DeepEqual(got, second) {
		t.Fatalf("模板更新不是整份替换: got=%v want=%v", got, second)
	}
}

// TestRouteTemplateModelRulesContainNoDeploymentData 验证模板拒绝入口和不属于模型的部署引用。
// 入口不能写入规则；独立权重必须引用该公开模型的真实部署，否则返回 400。
func TestRouteTemplateModelRulesContainNoDeploymentData(t *testing.T) {
	h := newHarness(t)
	admin := h.adminSession()
	for _, extra := range []map[string]any{
		{"endpoint_id": "chat"},
		{"allocations": []any{map[string]any{"deployment_id": "a", "weight": 1}}},
		{"deployment_id": "a"},
	} {
		rule := map[string]any{"model": "shared", "strategy": "traffic-split"}
		for key, value := range extra {
			rule[key] = value
		}
		body := routeTemplateBody([]any{rule}, 1, 60, 3, 0)
		if r := h.do(http.MethodPost, "/route_template/new", admin, map[string]any{"name": "invalid", "body": body}); r.status != http.StatusBadRequest {
			t.Fatalf("模板接受了部署级字段 %v: %s", extra, r.describe())
		}
	}
}

// TestRouteTemplateDuplicateModelsAreRejected 验证一个模板内每个公开模型最多一条规则。
func TestRouteTemplateDuplicateModelsAreRejected(t *testing.T) {
	h := newHarness(t)
	admin := h.adminSession()
	body := routeTemplateBody([]any{
		map[string]any{"model": "shared", "strategy": "random"},
		map[string]any{"model": "shared", "strategy": "least-busy"},
	}, 1, 60, 3, 0)
	if r := h.do(http.MethodPost, "/route_template/new", admin, map[string]any{"name": "duplicate", "body": body}); r.status != http.StatusBadRequest {
		t.Fatalf("重复公开模型未拒绝: %s", r.describe())
	}
}

// TestRouteTemplateRejectsTopLevelStrategy 验证创建、更新均拒绝顶层默认策略，拒绝后原正文不变。
// 前置隔离数据库与管理员；合法模型规则可保存回读；harness 清理模板与身份，不依赖供应商。
func TestRouteTemplateRejectsTopLevelStrategy(t *testing.T) {
	h := newHarness(t)
	admin := h.adminSession()
	body := routeTemplateBody([]any{}, 2, 60, 3, 60)
	id := routeTemplate(t, h, admin, "strict strategy", body)
	for _, raw := range []any{"simple-shuffle", "cost-based-routing", "", nil, 1} {
		invalid := routeTemplateBody([]any{}, 2, 60, 3, 60)
		invalid["routing_strategy"] = raw
		for _, path := range []string{"/route_template/new", "/route_template/" + id + "/update"} {
			r := h.do(http.MethodPost, path, admin, map[string]any{"name": "invalid", "body": invalid})
			if r.status != http.StatusBadRequest {
				t.Fatalf("顶层策略未返回400: %s", r.describe())
			}
		}
		if got := templateBody(t, h, admin, id); !reflect.DeepEqual(got, body) {
			t.Fatalf("拒绝写入改变正文: %v", got)
		}
	}
}
