package regression

import (
	"net/http"
	"strings"
	"testing"
)

// TestEnterpriseProvisioningAndArchive 前置真实网关与私有 PostgreSQL schema、本地推理供应商。
// 验证独立 SSO 配置、SCIM 同步到真实用户、密钥放行与停用拦截、归档入队和权限隔离；结束关闭网关并删除 schema。
func TestEnterpriseProvisioningAndArchive(t *testing.T) {
	h := newHarness(t, chatDeployment("enterprise-model"))
	// 本用例测试加密持久化，显式提供足够长度的测试加密密钥，不改变其他回归夹具的鉴权策略。
	h.gw.Cfg.GeneralSettings.MasterKey = strings.Repeat("e", 32)
	admin := h.adminSession()
	org := h.ok(http.MethodPost, "/organization/new", admin, map[string]any{"organization_alias": "Enterprise regression"}).json()
	organizationID := stringField(org, "organization_id")
	sso := h.ok(http.MethodPost, "/admin/enterprise/sso", admin, map[string]any{"organization_id": organizationID, "name": "Independent login", "issuer": "https://identity.example.com", "client_id": "xhub", "client_secret": "test-secret", "redirect_uri": "https://console.example.com/auth/enterprise/callback", "return_uri": "https://console.example.com/ui/login", "identity_claim": "sub", "enabled": false}).json()
	connection := stringField(sso, "connection_id")
	directory := h.ok(http.MethodPost, "/admin/directory/connections", admin, map[string]any{"organization_id": organizationID, "name": "Independent sync"}).json()
	token := stringField(directory, "token")
	base := "/scim/v2/connections/" + connection
	if got := h.do(http.MethodGet, base+"/Users", token, nil); got.status != 401 {
		t.Fatal("未启用目录允许同步", got.text())
	}
	h.ok(http.MethodPut, "/admin/directory/connections/"+connection, admin, map[string]any{"enabled": true})
	user := h.ok(http.MethodPost, base+"/Users", token, map[string]any{"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:User"}, "externalId": "employee-regression", "userName": "enterprise@example.com", "displayName": "Employee", "active": true}).json()
	detail := h.ok(http.MethodGet, "/admin/directory/connections/"+connection, admin, nil).json()
	rows := detail["users"].([]any)
	userID := stringField(rows[0].(map[string]any), "user_id")
	// SCIM 不授予本地消费额度，管理员显式批准预算后再验证密钥放行。
	h.setUserBudget(t, admin, userID, 10)
	key := stringField(h.ok(http.MethodPost, "/key/generate", admin, map[string]any{"user_id": userID, "key_alias": "enterprise-regression"}).json(), "key")
	archive := map[string]any{"bucket": "xhub-test-only", "region": "z0", "prefix": "billing", "access_key": "fake-ak", "secret_key": "fake-sk", "enabled": true, "interval_seconds": 86400, "batch_size": 1}
	h.ok(http.MethodPut, "/admin/archive/kodo", admin, archive)
	h.ok(http.MethodPost, "/v1/chat/completions", key, map[string]any{"model": "enterprise-model", "messages": []any{map[string]any{"role": "user", "content": "Verify enterprise provisioning"}}})
	h.gw.FlushSpend()
	status := h.ok(http.MethodGet, "/admin/archive/kodo", admin, nil)
	if strings.Contains(status.text(), "fake-sk") || len(status.json()["events"].([]any)) != 1 {
		t.Fatal("归档秘密泄漏或真实计费未入队", status.text())
	}
	h.ok(http.MethodPatch, base+"/Users/"+stringField(user, "id"), token, map[string]any{"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"}, "Operations": []any{map[string]any{"op": "replace", "path": "active", "value": false}}})
	denied := h.do(http.MethodPost, "/v1/chat/completions", key, map[string]any{"model": "enterprise-model", "messages": []any{map[string]any{"role": "user", "content": "Must be blocked"}}})
	if denied.status != 401 && denied.status != 403 {
		t.Fatal("SCIM 停用没有拦截真实密钥", denied.status, denied.text())
	}
	if got := h.do(http.MethodGet, "/admin/archive/kodo", token, nil); got.status != 401 {
		t.Fatal("SCIM 凭据越权读取存储配置", got.status)
	}
	h.ok(http.MethodDelete, "/admin/directory/connections/"+connection, admin, nil)
	h.ok(http.MethodGet, "/admin/enterprise/sso/"+connection, admin, nil)
	h.ok(http.MethodDelete, "/admin/enterprise/sso/"+connection, admin, nil)
	if got := h.do(http.MethodGet, base+"/Users", token, nil); got.status != 401 {
		t.Fatal("已删除连接的凭据仍有效")
	}
}
