package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sunqirui1987/xhub/internal/iam"
)

// TestEnterpriseIndependentHTTP 使用真实 PostgreSQL 私有 schema 和完整网关路由验证独立 SSO/SCIM 生命周期。
// 前置管理员会话；覆盖权限、输入失败、秘密保护、同组织锚点复用、双向删除与重新连接；夹具关闭服务器并删除 schema。
func TestEnterpriseIndependentHTTP(t *testing.T) {
	cfg, st, db := testGatewayStores(t)
	cfg.GeneralSettings.MasterKey = strings.Repeat("m", 32)
	ctx := context.Background()
	admin, err := db.CreateUser(ctx, testActor, iam.UserInput{Email: "independent-admin@example.com", Password: "password123", Role: iam.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	member, err := db.CreateUser(ctx, testActor, iam.UserInput{Email: "independent-member@example.com", Password: "password123", Role: iam.RoleUser})
	if err != nil {
		t.Fatal(err)
	}
	org, err := db.CreateOrg(ctx, testActor, "Independent HTTP", nil)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(New(cfg, st, db).Handler())
	t.Cleanup(srv.Close)
	session := loginAs(t, srv.URL, admin.Email, "password123")
	userSession := loginAs(t, srv.URL, member.Email, "password123")
	path := "/admin/enterprise/sso"
	// 请求辅助仅序列化输入并验证真实状态码，失败消息标明具体协议行为，不模拟服务或持久化。
	request := func(key, method, route string, data any, want int) []byte {
		t.Helper()
		var body []byte
		if data != nil {
			body, err = json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
		}
		status, out := authed(t, srv.URL, key, method, route, body)
		if status != want {
			t.Fatalf("独立配置接口 %s %s: got=%d want=%d body=%s", method, route, status, want, out)
		}
		return out
	}
	input := map[string]any{"organization_id": org.ID, "name": "Login only", "issuer": "https://identity.example.com", "client_id": "xhub", "client_secret": "independent-secret", "redirect_uri": "https://console.example.com/auth/enterprise/callback", "return_uri": "https://console.example.com/ui/login", "identity_claim": "sub", "enabled": false}
	request("", http.MethodGet, path, nil, 401)
	request(userSession, http.MethodPost, path, input, 403)
	status, out := authed(t, srv.URL, session, http.MethodPost, path, []byte("{} {}"))
	if status != 400 {
		t.Fatalf("尾随 JSON 未拒绝: %d %s", status, out)
	}
	for _, bad := range []struct {
		field  string
		value  any
		status int
	}{
		{"name", "   ", 400}, {"organization_id", "missing", 404},
		{"issuer", "http://identity.example.com", 400}, {"redirect_uri", "https://console.example.com/wrong", 400},
		{"return_uri", "https://other.example.com/ui/login", 400}, {"identity_claim", "email.value", 400},
	} {
		copy := make(map[string]any, len(input))
		for k, v := range input {
			copy[k] = v
		}
		copy[bad.field] = bad.value
		request(session, http.MethodPost, path, copy, bad.status)
	}
	created := request(session, http.MethodPost, path, input, 201)
	var sso iam.EnterpriseOIDC
	if err := json.Unmarshal(created, &sso); err != nil || sso.ConnectionID == "" || sso.Name != "Login only" {
		t.Fatalf("SSO 创建契约错误: %s %v", created, err)
	}
	if bytes.Contains(created, []byte("independent-secret")) {
		t.Fatal("创建 SSO 回显秘密")
	}
	request(session, http.MethodPost, path, input, 409)
	if rows := request(session, http.MethodGet, "/admin/directory/connections", nil, 200); string(bytes.TrimSpace(rows)) != "[]" {
		t.Fatalf("SSO 创建了 SCIM 配置: %s", rows)
	}
	route := path + "/" + sso.ConnectionID
	// 匿名入口必须由真实启用状态控制；仅公开名称和受控路径，绝不公开凭据或身份平台详情。
	if out := request("", http.MethodGet, "/auth/enterprise/options", nil, 200); string(bytes.TrimSpace(out)) != "[]" {
		t.Fatalf("停用的 SSO 出现在登录入口: %s", out)
	}
	input["enabled"] = true
	request(session, http.MethodPut, route, input, 400)
	// 此用例只验证入口查询和持久化；无法 discovery 必须拒绝启用，成功协议另由 TLS fixture 验证。
	if _, err := db.Pool().Exec(ctx, "UPDATE enterprise_oidc SET enabled=true WHERE connection_id=$1", sso.ConnectionID); err != nil {
		t.Fatal(err)
	}
	var options []map[string]string
	public := request("", http.MethodGet, "/auth/enterprise/options", nil, 200)
	if err := json.Unmarshal(public, &options); err != nil || len(options) != 1 {
		t.Fatalf("启用 SSO 后匿名入口契约错误: %s %v", public, err)
	}
	if len(options[0]) != 3 || options[0]["id"] != sso.ConnectionID || options[0]["name"] != "Login only" || options[0]["url"] != "/gw/auth/enterprise/"+sso.ConnectionID+"/start" {
		t.Fatalf("公开 SSO 入口泄漏额外字段或授权地址错误: %s", public)
	}
	input["enabled"] = false
	request(session, http.MethodPut, route, input, 200)
	if out := request("", http.MethodGet, "/auth/enterprise/options", nil, 200); string(bytes.TrimSpace(out)) != "[]" {
		t.Fatalf("再次停用 SSO 后仍提供入口: %s", out)
	}
	// 新系统仅保留独立 SSO 路由，禁止通过历史 SCIM 子路径读写登录配置。
	legacy := "/admin/directory/connections/" + sso.ConnectionID + "/oidc"
	request(session, http.MethodGet, legacy, nil, 404)
	request(session, http.MethodPut, legacy, input, 404)
	stored, err := db.EnterpriseOIDCByID(ctx, sso.ConnectionID)
	if err != nil {
		t.Fatal(err)
	}
	secret := stored.Secret
	if plain, err := enterpriseSecret(cfg.GeneralSettings.MasterKey, secret, true); err != nil || plain != "independent-secret" {
		t.Fatalf("SSO 秘密未加密持久化: %v", err)
	}
	for _, route := range []string{path, route} {
		read := request(session, http.MethodGet, route, nil, 200)
		if bytes.Contains(read, []byte(secret)) || bytes.Contains(read, []byte("independent-secret")) {
			t.Fatal("读取 SSO 回显秘密")
		}
	}
	req, _ := http.NewRequest(http.MethodGet, srv.URL+route, nil)
	req.Header.Set("Authorization", "Bearer "+session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("SSO 管理响应允许缓存")
	}
	// SCIM 独立创建名称和凭据，内部同组织锚点复用，不改变 SSO 的任何配置。
	var sync struct {
		Connection iam.DirectoryConnection `json:"connection"`
		Token      string                  `json:"token"`
	}
	if err := json.Unmarshal(request(session, http.MethodPost, "/admin/directory/connections", map[string]any{"organization_id": org.ID, "name": "Sync only"}, 201), &sync); err != nil {
		t.Fatal(err)
	}
	if sync.Connection.ID != sso.ConnectionID || sync.Token == "" || sync.Connection.Name != "Sync only" {
		t.Fatalf("独立 SCIM 创建错误: %+v", sync.Connection)
	}
	request(sync.Token, http.MethodGet, route, nil, 401)
	delete(input, "name")
	delete(input, "client_secret")
	request(session, http.MethodPut, route, input, 400)
	input["name"] = "Login only"
	request(session, http.MethodPut, route, input, 200)
	stored, err = db.EnterpriseOIDCByID(ctx, sso.ConnectionID)
	if err != nil || stored.Name != "Login only" || stored.Secret != secret {
		t.Fatalf("省略秘密覆盖原配置: %+v %v", stored, err)
	}
	base := "/scim/v2/connections/" + sso.ConnectionID
	request(session, http.MethodPut, "/admin/directory/connections/"+sso.ConnectionID, map[string]any{"enabled": true}, 200)
	request(sync.Token, http.MethodGet, base+"/Users", nil, 200)
	du, local, err := db.ProvisionDirectoryUser(ctx, sso.ConnectionID, "stable-employee", "independent-employee@example.com", "Employee", true)
	if err != nil {
		t.Fatal(err)
	}
	request(session, http.MethodDelete, route, nil, 204)
	request(session, http.MethodDelete, route, nil, 404)
	request(session, http.MethodGet, route, nil, 404)
	request(session, http.MethodPut, route, input, 404)
	request(sync.Token, http.MethodGet, base+"/Users/"+du.ID, nil, 200)
	input["name"] = "Reconnected login"
	request(session, http.MethodPost, path, input, 201)
	request(session, http.MethodDelete, "/admin/directory/connections/"+sso.ConnectionID, nil, 204)
	request(sync.Token, http.MethodGet, base+"/Users", nil, 401)
	request(session, http.MethodGet, route, nil, 200)
	request(session, http.MethodGet, "/user/info?user_id="+local.ID, nil, 200)
	// 重新连接同步签发新 token 并复用原稳定身份，旧 token 必须保持撤销。
	oldToken := sync.Token
	if err := json.Unmarshal(request(session, http.MethodPost, "/admin/directory/connections", map[string]any{"organization_id": org.ID, "name": "Reconnected sync"}, 201), &sync); err != nil {
		t.Fatal(err)
	}
	if sync.Connection.ID != sso.ConnectionID || sync.Token == oldToken {
		t.Fatal("重连丢失稳定锚点或复用旧凭据")
	}
	request(session, http.MethodPut, "/admin/directory/connections/"+sso.ConnectionID, map[string]any{"enabled": true}, 200)
	request(oldToken, http.MethodGet, base+"/Users", nil, 401)
	request(sync.Token, http.MethodGet, base+"/Users/"+du.ID, nil, 200)
}
