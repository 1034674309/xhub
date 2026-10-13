package regression

import (
	"net/http"
	"testing"
)

// TestUserAccountEditing 验证真实用户更新接口和团队关系；前置隔离网关、管理员与两个成员，参数 t 为测试上下文，无返回值。
// 覆盖更新契约、空名称/预算、非法字段回滚、普通成员越权、团队角色、离队后个人推理与计费、成员关系清理及删除后密钥失效；harness 清理私有 schema 和本地上游。
func TestUserAccountEditing(t *testing.T) {
	h := newHarness(t, chatDeployment("edit-chat"))
	admin := h.adminSession()
	a := h.provision(t, admin, "edit-a")
	b := h.provision(t, admin, "edit-b")
	result := h.ok(http.MethodPost, "/user/update", admin, map[string]any{"user_id": a.userID, "user_email": "Edited@xhub.local", "user_alias": "编辑后的名称", "max_budget": 12.5, "user_role": "user"}).json()["user_info"].(map[string]any)
	if result["user_alias"] != "编辑后的名称" || result["user_email"] != "edited@xhub.local" || result["max_budget"] != 12.5 {
		t.Fatalf("用户更新契约错误: %+v", result)
	}
	for _, fields := range []map[string]any{{"user_role": "invalid"}, {"max_budget": -1}, {"user_email": ""}} {
		fields["user_id"], fields["user_alias"] = a.userID, "不能部分保存"
		if r := h.do(http.MethodPost, "/user/update", admin, fields); r.status != 400 {
			t.Fatalf("非法更新未返回 400: %s", r.describe())
		}
		current := h.ok(http.MethodGet, "/user/info?user_id="+a.userID, admin, nil).json()["user_info"].(map[string]any)
		if current["user_alias"] != "编辑后的名称" {
			t.Fatalf("非法更新未整体回滚: %+v", current)
		}
	}
	if r := h.do(http.MethodPost, "/user/update", b.session, map[string]any{"user_id": a.userID, "user_alias": "越权"}); r.status != 403 && r.status != 404 {
		t.Fatalf("普通成员可编辑他人: %s", r.describe())
	}
	h.ok(http.MethodPost, "/user/update", admin, map[string]any{"user_id": a.userID, "user_alias": "", "max_budget": nil})
	empty := h.ok(http.MethodGet, "/v2/user/info?user_id="+a.userID, admin, nil).json()["user_info"].(map[string]any)
	if empty["user_alias"] != "" || empty["max_budget"] != nil {
		t.Fatalf("空名称/无限预算未保存: %+v", empty)
	}
	h.ok(http.MethodPost, "/team/member_update", admin, map[string]any{"team_id": a.teamID, "user_id": a.userID, "role": "admin"})
	h.ok(http.MethodPost, "/team/member_update", admin, map[string]any{"team_id": a.teamID, "user_id": a.userID, "role": "user"})
	h.ok(http.MethodPost, "/v1/chat/completions", a.key, map[string]any{"model": "edit-chat", "messages": []any{map[string]any{"role": "user", "content": "before removal"}}})
	h.ok(http.MethodPost, "/team/member_delete", admin, map[string]any{"team_id": a.teamID, "user_id": a.userID})
	// Key 只持久化个人归属，离队不撤销凭证；空个人预算代表不限额，后续消费必须派生为个人归属。
	callsBefore := len(h.upstreamCalls())
	h.ok(http.MethodPost, "/v1/chat/completions", a.key, map[string]any{"model": "edit-chat", "messages": []any{map[string]any{"role": "user", "content": "after removal"}}})
	if got := len(h.upstreamCalls()); got != callsBefore+1 {
		t.Fatalf("离队后的个人推理必须调用一次上游：调用次数从 %d 变为 %d", callsBefore, got)
	}
	h.flushSpend()
	events, err := h.db.Engine.QueryString("SELECT user_id, team_id, organization_id FROM usage_events WHERE user_id = ? AND cost > 0", a.userID)
	if err != nil || len(events) != 2 {
		t.Fatalf("离队前后成功推理必须各记录一次计费用量: events=%v err=%v", events, err)
	}
	teamEvents, personalEvents := 0, 0
	for _, event := range events {
		if event["user_id"] != a.userID {
			t.Fatalf("离队改变了消费用户归属: %v", event)
		}
		switch event["team_id"] {
		case a.teamID:
			teamEvents++
		case "":
			personalEvents++
			if event["organization_id"] != "" {
				t.Fatalf("离队后的个人消费仍归属原组织: %v", event)
			}
		default:
			t.Fatalf("消费被记录到其他团队: %v", event)
		}
	}
	if teamEvents != 1 || personalEvents != 1 {
		t.Fatalf("离队不能改写历史账单或继续向原团队记账: team=%d personal=%d", teamEvents, personalEvents)
	}
	// 此夹具另有独立组织成员关系，离开团队不等于离开组织；直接验证团队成员关系已删除。
	members, err := h.db.ListMembers(t.Context(), a.teamID)
	if err != nil {
		t.Fatalf("读取离队后的团队成员失败: %v", err)
	}
	for _, member := range members {
		if member.UserID == a.userID {
			t.Fatalf("离队后仍保留团队成员权限: %+v", member)
		}
	}
	h.ok(http.MethodPost, "/user/delete", admin, map[string]any{"user_id": a.userID})
	if r := h.do(http.MethodGet, "/user/info?user_id="+a.userID, admin, nil); r.status != 404 {
		t.Fatalf("删除后账户仍存在: %s", r.describe())
	}
	// 删除账户才撤销个人凭证；有效请求应在访问上游之前被鉴权拒绝。
	callsBefore = len(h.upstreamCalls())
	if r := h.do(http.MethodPost, "/v1/chat/completions", a.key, map[string]any{"model": "edit-chat", "messages": []any{map[string]any{"role": "user", "content": "after deletion"}}}); r.status != 401 {
		t.Fatalf("删除账户后个人密钥仍有效: %s", r.describe())
	}
	if got := len(h.upstreamCalls()); got != callsBefore {
		t.Fatalf("删除账户后的拒绝请求仍调用上游：调用次数从 %d 变为 %d", callsBefore, got)
	}
}
