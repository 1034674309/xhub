package regression

import (
	"net/http"
	"reflect"
	"testing"
)

// TestOrganizationConfigRoundTrip 验证组织创建和修改确实保存模型范围与任意 JSON 对象元数据。
// 前置真实后台及隔离 PostgreSQL；核对创建、详情、列表、部分更新、清空及非法输入回滚，harness 清理 schema。
func TestOrganizationConfigRoundTrip(t *testing.T) {
	h := newHarness(t, chatDeployment("org-alpha"), chatDeployment("org-beta"))
	admin := h.adminSession()
	meta := map[string]any{"department": "研发试点", "enabled": true, "nested": map[string]any{"limit": float64(3)}}
	created := h.ok(http.MethodPost, "/organization/new", admin, map[string]any{
		"organization_alias": "config-roundtrip", "models": []string{"org-alpha"}, "metadata": meta,
	}).json()
	id := stringField(created, "organization_id")
	if !reflect.DeepEqual(created["models"], []any{"org-alpha"}) || !reflect.DeepEqual(created["metadata"], meta) {
		t.Fatalf("组织创建丢失模型或元数据: %v", created)
	}
	for _, path := range []string{"/organization/info?organization_id=" + id, "/organization/list"} {
		response := h.ok(http.MethodGet, path, admin, nil)
		row := response.json()
		if path == "/organization/list" {
			rows := rowsOf(response)
			if len(rows) != 1 {
				t.Fatalf("新组织应在列表中唯一出现: %s", response.describe())
			}
			row = rows[0]
		}
		if !reflect.DeepEqual(row["models"], created["models"]) || !reflect.DeepEqual(row["metadata"], meta) {
			t.Fatalf("组织回读丢失配置 %s: %v", path, row)
		}
	}
	h.ok(http.MethodPatch, "/organization/update", admin, map[string]any{"organization_id": id, "organization_alias": "renamed"})
	for _, invalid := range []map[string]any{
		{"models": "org-beta"}, {"models": []any{4}}, {"models": []any{""}},
		{"models": []any{"all-proxy-models", "org-alpha"}}, {"metadata": []any{}}, {"metadata": "invalid"},
	} {
		invalid["organization_id"] = id
		if r := h.do(http.MethodPatch, "/organization/update", admin, invalid); r.status != http.StatusBadRequest {
			t.Fatalf("非法组织配置应拒绝且不改变原数据: input=%v %s", invalid, r.describe())
		}
	}
	read := h.ok(http.MethodGet, "/organization/info?organization_id="+id, admin, nil).json()
	if !reflect.DeepEqual(read["models"], created["models"]) || !reflect.DeepEqual(read["metadata"], meta) || read["organization_alias"] != "renamed" {
		t.Fatalf("部分更新或失败请求覆盖原配置: %v", read)
	}
	cleared := h.ok(http.MethodPatch, "/organization/update", admin, map[string]any{"organization_id": id, "models": []any{}, "metadata": nil}).json()
	if !reflect.DeepEqual(cleared["models"], []any{}) || !reflect.DeepEqual(cleared["metadata"], map[string]any{}) {
		t.Fatalf("组织配置未正确清空: %v", cleared)
	}
}

// TestOrganizationModelCeilingEnforced 验证组织上限立即约束已有团队、项目和密钥，列表与真实推理一致。
// 前置两个可调用本地上游模型及隔离租户；验证收窄、越权修改、清空继承、无上游副作用与跨组织隔离，harness 清理全部资源。
func TestOrganizationModelCeilingEnforced(t *testing.T) {
	h := newHarness(t, chatDeployment("org-alpha"), chatDeployment("org-beta"))
	admin := h.adminSession()
	owner := h.openScope(t, admin, "org-ceiling")
	other := h.openScope(t, admin, "org-other")
	h.ok(http.MethodPost, "/v1/chat/completions", owner.key, chatRequest("org-beta", "Before organization restriction"))
	h.ok(http.MethodPatch, "/organization/update", admin, map[string]any{"organization_id": owner.orgID, "models": []string{"org-alpha"}})
	before := len(h.upstreamCalls())
	if r := h.do(http.MethodPost, "/v1/chat/completions", owner.key, chatRequest("org-beta", "Blocked by organization")); r.status != http.StatusUnauthorized {
		t.Fatalf("组织模型上限未拦截已有密钥: %s", r.describe())
	}
	if len(h.upstreamCalls()) != before {
		t.Fatal("组织拒绝的请求仍调用了上游")
	}
	models := namesOf(rowsOf(h.ok(http.MethodGet, "/v1/models", owner.key, nil), "data"), "id")
	if !reflect.DeepEqual(models, []string{"org-alpha"}) {
		t.Fatalf("模型列表未应用组织上限: %v", models)
	}
	for _, credential := range []string{owner.session, owner.key} {
		for _, suffix := range []string{"", "?team_id=" + other.teamID} {
			visible := namesOf(rowsOf(h.ok(http.MethodGet, "/v1/models"+suffix, credential, nil), "data"), "id")
			if !reflect.DeepEqual(visible, []string{"org-alpha"}) {
				t.Fatalf("会话或伪造团队绕过模型上限: %v", visible)
			}
		}
	}
	h.ok(http.MethodPost, "/v1/chat/completions", owner.key, chatRequest("org-alpha", "Allowed by organization"))
	h.ok(http.MethodPost, "/v1/chat/completions", other.key, chatRequest("org-beta", "Other organization remains independent"))
	if r := h.do(http.MethodPatch, "/organization/update", owner.session, map[string]any{"organization_id": owner.orgID, "models": []string{"org-beta"}}); r.status != http.StatusForbidden {
		t.Fatalf("组织管理员不应扩展平台设定的模型上限: %s", r.describe())
	}
	h.ok(http.MethodPatch, "/organization/update", owner.session, map[string]any{"organization_id": owner.orgID, "metadata": map[string]any{"owner": "department"}})
	h.ok(http.MethodPatch, "/organization/update", admin, map[string]any{"organization_id": owner.orgID, "models": []string{}})
	h.ok(http.MethodPost, "/v1/chat/completions", owner.key, chatRequest("org-beta", "Organization restriction cleared"))
}

// TestOrganizationModelWriteBoundaries 验证团队、项目和密钥不能保存越界模型；前置隔离后台与本地模型。
// 参数 t 为测试上下文，无返回；核对字段回读、跨组织拒绝和失败后无模型配置副作用，harness 删除私有 schema。
func TestOrganizationModelWriteBoundaries(t *testing.T) {
	h := newHarness(t, chatDeployment("org-alpha"), chatDeployment("org-beta"))
	admin := h.adminSession()
	owner := h.openScope(t, admin, "boundary-owner")
	other := h.openScope(t, admin, "boundary-other")
	h.ok(http.MethodPatch, "/organization/update", admin, map[string]any{"organization_id": owner.orgID, "models": []string{"org-alpha"}})
	info := h.ok(http.MethodGet, "/team/info?team_id="+owner.teamID, owner.session, nil).json()["team_info"].(map[string]any)
	if !reflect.DeepEqual(info["organization_models"], []any{"org-alpha"}) {
		t.Fatalf("团队详情未返回组织上限: %v", info)
	}
	for _, tc := range []struct {
		path string
		body map[string]any
	}{
		{"/team/new", map[string]any{"organization_id": owner.orgID, "team_alias": "invalid-team", "models": []string{"org-beta"}}},
		{"/team/update", map[string]any{"team_id": owner.teamID, "models": []string{"org-beta"}}},
		{"/project/new", map[string]any{"team_id": owner.teamID, "project_alias": "invalid-project", "models": []string{"org-beta"}}},
		{"/key/generate", map[string]any{"user_id": owner.userID, "models": []string{"org-beta"}}},
	} {
		r := h.do(http.MethodPost, tc.path, admin, tc.body)
		if r.status != http.StatusBadRequest {
			t.Fatalf("组织上限越界写入未拒绝 %s: %s", tc.path, r.describe())
		}
	}
	if r := h.do(http.MethodPatch, "/organization/update", other.session, map[string]any{"organization_id": owner.orgID, "metadata": map[string]any{"foreign": true}}); r.status != http.StatusForbidden {
		t.Fatalf("跨组织元数据修改未拒绝: %s", r.describe())
	}
	read := h.ok(http.MethodGet, "/organization/info?organization_id="+owner.orgID, admin, nil).json()
	if !reflect.DeepEqual(read["metadata"], map[string]any{}) {
		t.Fatalf("拒绝请求改变组织元数据: %v", read)
	}
}
