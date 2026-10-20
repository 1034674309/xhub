package regression

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// TestKeyLifecycleSettingsContract 验证真实后台创建、编辑、轮换和模型调用；前置私有数据库与本地上游。
// 确认期限恢复、预算/限流回读、非法更新不撤销旧 Key、立即撤销契约以及不支持设置明确失败；harness 清理 schema。
func TestKeyLifecycleSettingsContract(t *testing.T) {
	h := newHarness(t, chatDeployment("lifecycle-chat"))
	admin := h.adminSession()
	h.setUserBudget(t, admin, firstString(h.ok(http.MethodGet, "/auth/me", admin, nil).json(), "user_id"), 100)
	created := h.ok(http.MethodPost, "/key/generate", admin, map[string]any{"key_type": "llm_api", "key_alias": "lifecycle"}).json()
	id, secret := firstString(created, "token_id"), firstString(created, "key")
	for _, raw := range []string{`{"duration":`, "[]", "{} trailing"} {
		got := h.doRaw(http.MethodPost, "/key/"+id+"/regenerate", admin, "application/json", raw)
		if got.status != 400 {
			t.Fatalf("坏轮换正文未拒绝: %s", got.describe())
		}
		h.ok(http.MethodGet, "/v1/models", secret, nil)
	}
	for _, fields := range []map[string]any{{"key_type": "management"}, {"key_type": "default"}, {"key_type": "read_only"}, {"auto_rotate": true, "rotation_interval": "30d"}, {"allowed_routes": []string{"management_routes"}}, {"grace_period": "60s"}, {"max_budget": -1}, {"duration": "invalid"}} {
		for _, path := range []string{"/key/generate", "/key/update", "/key/" + id + "/regenerate"} {
			fields["key"] = id
			if got := h.do(http.MethodPost, path, admin, fields); got.status != 400 {
				t.Fatalf("不支持或非法设置未拒绝: path=%s result=%s", path, got.describe())
			}
		}
		h.ok(http.MethodGet, "/v1/models", secret, nil)
	}
	h.ok(http.MethodPost, "/key/update", admin, map[string]any{"key": id, "key_type": "llm_api", "auto_rotate": false, "rotation_interval": ""})
	// 数据库只在本测试中设为过去，避免依赖定时等待；之后所有验证走 HTTP。
	if _, err := h.db.Engine.Exec("UPDATE api_keys SET expires_at = ? WHERE id = ?", time.Now().Add(-time.Hour), id); err != nil {
		t.Fatal(err)
	}
	if got := h.do(http.MethodGet, "/v1/models", secret, nil); got.status != 401 {
		t.Fatalf("过期 Key 未拒绝: %s", got.describe())
	}
	if got := h.do(http.MethodPost, "/key/"+id+"/regenerate", admin, map[string]any{}); got.status != 400 {
		t.Fatalf("过期轮换未要求期限: %s", got.describe())
	}
	renewed := h.ok(http.MethodPost, "/key/"+id+"/regenerate", admin, map[string]any{"key_alias": "renewed", "duration": "1h", "max_budget": 1.0, "rpm_limit": 5, "tpm_limit": 10000}).json()
	next := firstString(renewed, "key")
	if renewed["max_budget"] != 1.0 || renewed["rpm_limit"] != 5.0 || renewed["tpm_limit"] != 10000.0 || renewed["key_alias"] != "renewed" {
		t.Fatalf("轮换响应未持久化表单: %v", renewed)
	}
	k, err := h.db.GetKey(context.Background(), id)
	if err != nil || k.ExpiresAt == nil || k.ExpiresAt.Before(time.Now().Add(59*time.Minute)) || k.MaxBudget == nil || *k.MaxBudget != 1 {
		t.Fatalf("轮换期限/预算回读错误: %+v %v", k, err)
	}
	h.ok(http.MethodPost, "/v1/chat/completions", next, chatRequest("lifecycle-chat", "Confirm renewed key access."))
	if got := h.do(http.MethodGet, "/v1/models", secret, nil); got.status != 401 {
		t.Fatalf("旧 Key 没有立即撤销: %s", got.describe())
	}
	// 将 RPM 设为零后必须在供应商调用前拒绝，证明轮换保存的不只是展示字段。
	zero := h.ok(http.MethodPost, "/key/"+id+"/regenerate", admin, map[string]any{"rpm_limit": 0}).json()
	if got := h.do(http.MethodPost, "/v1/chat/completions", firstString(zero, "key"), chatRequest("lifecycle-chat", "Must be blocked.")); got.status != 429 {
		t.Fatalf("轮换零 RPM 未生效: %s", got.describe())
	}
	h.ok(http.MethodPost, "/key/delete", admin, map[string]any{"keys": []string{id}})
	if got := h.do(http.MethodGet, "/v1/models", firstString(zero, "key"), nil); got.status != 401 {
		t.Fatalf("删除凭据仍可使用: %s", got.describe())
	}
}
