package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sunqirui1987/xhub/internal/iam"
)

// TestSCIMPaging 验证列表参数正常、边界和非法输入；前置为无存储 HTTP 请求。
// 验证精确过滤、零数量、上限和不支持表达式；无数据副作用，不需清理。
func TestSCIMPaging(t *testing.T) {
	for _, tc := range []struct {
		query string
		valid bool
		count int
	}{
		{"", true, 100}, {"count=0", true, 0}, {"count=200&startIndex=2", true, 200},
		{"filter=userName+eq+%22member%40example.com%22", true, 100},
		{"filter=externalId+eq+%22employee-1%22", true, 100},
		{"count=-1", false, 0}, {"count=201", false, 0}, {"startIndex=0", false, 0},
		{"count=bad", false, 0}, {"filter=userName+co+%22test%22", false, 0},
		{"filter=unknown+eq+%22test%22", false, 0}, {"sortBy=userName", false, 0},
	} {
		t.Run(tc.query, func(t *testing.T) {
			_, count, _, _, err := scimPaging(httptest.NewRequest(http.MethodGet, "/Users?"+tc.query, nil))
			if (err == nil) != tc.valid || (tc.valid && count != tc.count) {
				t.Fatalf("分页或过滤契约不符: count=%d err=%v", count, err)
			}
		})
	}
}

// TestDirectoryHTTP 验证真实管理和 SCIM 路由；前置为隔离数据库、管理员登录和真实网关。
// 覆盖凭据、用户同步、分页、过滤、跨连接、停用与错误 schema；夹具清理服务器和 schema。
func TestDirectoryHTTP(t *testing.T) {
	cfg, st, db := testGatewayStores(t)
	ctx := context.Background()
	admin, err := db.CreateUser(ctx, testActor, iam.UserInput{Email: "directory-admin@example.com", Password: "password123", Role: iam.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	org, err := db.CreateOrg(ctx, testActor, "Directory HTTP company", nil)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(New(cfg, st, db).Handler())
	t.Cleanup(srv.Close)
	sess := loginAs(t, srv.URL, admin.Email, "password123")
	createBody, _ := json.Marshal(map[string]any{"organization_id": org.ID, "name": "Directory"})
	status, body := authed(t, srv.URL, sess, http.MethodPost, "/admin/directory/connections", createBody)
	if status != 201 {
		t.Fatalf("管理员创建连接失败: %d %s", status, body)
	}
	var created struct {
		Connection iam.DirectoryConnection `json:"connection"`
		Token      string                  `json:"token"`
	}
	if err := json.Unmarshal(body, &created); err != nil || created.Token == "" {
		t.Fatalf("一次性凭据缺失: %s %v", body, err)
	}
	base := "/scim/v2/connections/" + created.Connection.ID
	status, body = authed(t, srv.URL, created.Token, http.MethodGet, "/admin/directory/connections/"+created.Connection.ID, nil)
	if status == 200 {
		t.Fatalf("同步凭据读取管理快照: %s", body)
	}
	status, body = authed(t, srv.URL, created.Token, http.MethodGet, base+"/Users", nil)
	if status != 401 {
		t.Fatalf("未启用连接接受同步: %d %s", status, body)
	}
	status, body = authed(t, srv.URL, sess, http.MethodPut, "/admin/directory/connections/"+created.Connection.ID, []byte(`{"enabled":true}`))
	if status != 200 {
		t.Fatalf("连接启用失败: %d %s", status, body)
	}
	input := map[string]any{"schemas": []string{scimUserSchema}, "externalId": "employee-http-1", "userName": "directory-http@example.com", "displayName": "Directory Member", "active": true}
	inputBody, _ := json.Marshal(input)
	status, body = authed(t, srv.URL, created.Token, http.MethodPost, base+"/Users", inputBody)
	if status != 201 {
		t.Fatalf("同步创建失败: %d %s", status, body)
	}
	var user scimUser
	if err := json.Unmarshal(body, &user); err != nil || user.ID == "" {
		t.Fatalf("资源 ID 缺失: %s %v", body, err)
	}
	groupBody, _ := json.Marshal(scimGroup{Schemas: []string{scimGroupSchema}, ExternalID: "department-http", DisplayName: "Engineering", Members: []scimMember{{Value: user.ID, Type: "User"}}})
	status, body = authed(t, srv.URL, created.Token, http.MethodPost, base+"/Groups", groupBody)
	if status != 201 {
		t.Fatalf("组同步失败: %d %s", status, body)
	}
	var group scimGroup
	_ = json.Unmarshal(body, &group)
	status, body = authed(t, srv.URL, sess, http.MethodGet, "/admin/directory/connections/"+created.Connection.ID, nil)
	var snapshot iam.DirectoryManagement
	if err := json.Unmarshal(body, &snapshot); err != nil || status != 200 || len(snapshot.Users) != 1 || len(snapshot.Groups) != 1 || snapshot.Users[0].UserID == "" || snapshot.Users[0].Email != user.UserName {
		t.Fatalf("管理快照错误: %d %s %v", status, body, err)
	}
	if snapshot.Connection.TokenHash != "" {
		t.Fatal("快照泄露凭据哈希")
	}
	status, body = authed(t, srv.URL, created.Token, http.MethodGet, base+"/Groups/"+group.ID, nil)
	var fetchedGroup scimGroup
	_ = json.Unmarshal(body, &fetchedGroup)
	if status != 200 || len(fetchedGroup.Members) != 1 || fetchedGroup.Members[0].Value != user.ID {
		t.Fatalf("组成员读取错误: %d %s", status, body)
	}
	status, body = authed(t, srv.URL, created.Token, http.MethodGet, base+"/Groups?filter=displayName+eq+%22Engineering%22", nil)
	if status != 200 {
		t.Fatalf("组精确过滤失败: %d %s", status, body)
	}
	group.Members = []scimMember{{Value: "unknown-user"}}
	invalidGroup, _ := json.Marshal(group)
	status, body = authed(t, srv.URL, created.Token, http.MethodPut, base+"/Groups/"+group.ID, invalidGroup)
	if status != 404 {
		t.Fatalf("未知成员未拒绝: %d %s", status, body)
	}
	storedGroup, err := db.DirectoryGroupByID(ctx, created.Connection.ID, group.ID)
	if err != nil || len(storedGroup.Members) != 1 || storedGroup.Members[0] != user.ID {
		t.Fatalf("失败组同步未回滚: %+v %v", storedGroup, err)
	}
	status, body = authed(t, srv.URL, created.Token, http.MethodDelete, base+"/Groups/"+group.ID, nil)
	if status != 204 {
		t.Fatalf("组删除失败: %d %s", status, body)
	}
	status, body = authed(t, srv.URL, created.Token, http.MethodGet, base+"/Groups/"+group.ID, nil)
	if status != 404 {
		t.Fatalf("删除组仍可读取: %d %s", status, body)
	}
	status, body = authed(t, srv.URL, created.Token, http.MethodGet, base+"/Users/"+user.ID, nil)
	var read scimUser
	_ = json.Unmarshal(body, &read)
	if status != 200 || read.UserName != "directory-http@example.com" || read.ExternalID != "employee-http-1" {
		t.Fatalf("读取资源属性错误: %d %s", status, body)
	}
	status, body = authed(t, srv.URL, created.Token, http.MethodGet, base+"/Users?count=0", nil)
	var list struct {
		Total     int        `json:"totalResults"`
		Resources []scimUser `json:"Resources"`
	}
	_ = json.Unmarshal(body, &list)
	if status != 200 || list.Total != 1 || len(list.Resources) != 0 {
		t.Fatalf("零分页总数错误: %d %s", status, body)
	}
	status, body = authed(t, srv.URL, created.Token, http.MethodGet, base+"/Users?filter=userName+eq+%22directory-http%40example.com%22", nil)
	_ = json.Unmarshal(body, &list)
	if status != 200 || list.Total != 1 || len(list.Resources) != 1 || list.Resources[0].ID != user.ID {
		t.Fatalf("精确过滤失败: %d %s", status, body)
	}
	status, body = authed(t, srv.URL, created.Token, http.MethodGet, base+"/Users?filter=userName+co+%22http%22", nil)
	var failure struct {
		Schemas []string `json:"schemas"`
		Type    string   `json:"scimType"`
	}
	_ = json.Unmarshal(body, &failure)
	if status != 400 || failure.Type != "invalidFilter" || len(failure.Schemas) != 1 {
		t.Fatalf("未支持过滤未按 SCIM 拒绝: %d %s", status, body)
	}
	otherOrg, err := db.CreateOrg(ctx, testActor, "Other directory", nil)
	if err != nil {
		t.Fatal(err)
	}
	other, otherToken, err := db.CreateDirectoryConnection(ctx, testActor, otherOrg.ID, "Other")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ConfigureDirectory(ctx, testActor, other.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	status, body = authed(t, srv.URL, otherToken, http.MethodGet, "/scim/v2/connections/"+other.ID+"/Users/"+user.ID, nil)
	if status != 404 {
		t.Fatalf("跨连接读取未拒绝: %d %s", status, body)
	}
	status, body = authed(t, srv.URL, created.Token, http.MethodDelete, base+"/Users/"+user.ID, nil)
	if status != 204 {
		t.Fatalf("逻辑停用失败: %d %s", status, body)
	}
	stored, err := db.DirectoryUserByID(ctx, created.Connection.ID, user.ID)
	if err != nil || stored.Active {
		t.Fatalf("停用未持久化: %+v %v", stored, err)
	}
	local, err := db.GetUser(ctx, stored.UserID)
	if err != nil || local.Active() || local.SessionVersion != 2 {
		t.Fatalf("停用未撤销会话: %+v %v", local, err)
	}
	status, body = authed(t, srv.URL, created.Token, http.MethodGet, "/admin/directory/connections", nil)
	if status == 200 {
		t.Fatalf("同步凭据获得管理权限: %s", body)
	}
	// 同步凭据不能删除连接；管理员删除不需要 JSON 正文，旧凭据立即失效且保留停用用户。
	path := "/admin/directory/connections/" + created.Connection.ID
	status, body = authed(t, srv.URL, created.Token, http.MethodDelete, path, nil)
	if status != 401 {
		t.Fatalf("同步凭据可删除连接: %d %s", status, body)
	}
	status, body = authed(t, srv.URL, sess, http.MethodDelete, path, nil)
	if status != 204 || len(body) != 0 {
		t.Fatalf("删除契约错误: %d %s", status, body)
	}
	status, body = authed(t, srv.URL, sess, http.MethodGet, path, nil)
	if status != 404 {
		t.Fatalf("已删除连接仍有快照: %d %s", status, body)
	}
	status, body = authed(t, srv.URL, sess, http.MethodDelete, path, nil)
	if status != 404 {
		t.Fatalf("重复删除未返回不存在: %d %s", status, body)
	}
	status, body = authed(t, srv.URL, created.Token, http.MethodGet, base+"/Users", nil)
	if status != 401 {
		t.Fatalf("已删除连接凭据仍可同步: %d %s", status, body)
	}
	if kept, err := db.GetUser(ctx, stored.UserID); err != nil || kept.Status != local.Status {
		t.Fatalf("删除连接影响本地用户: %+v %v", kept, err)
	}
	if _, err := db.AuthenticateDirectory(ctx, other.ID, otherToken); err != nil {
		t.Fatalf("删除影响其他连接: %v", err)
	}
}
