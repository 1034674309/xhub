package gateway

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"github.com/sunqirui1987/xhub/internal/iam"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

// TestEnterpriseSecurity 验证密文、URL、SSRF 和响应上限；无网络前置，无持久化或清理需求。
func TestEnterpriseSecurity(t *testing.T) {
	master := strings.Repeat("m", 32)
	cipher, err := enterpriseSecret(master, "secret", false)
	if err != nil || cipher == "secret" {
		t.Fatal("密文生成失败", err)
	}
	plain, err := enterpriseSecret(master, cipher, true)
	if err != nil || plain != "secret" {
		t.Fatal("解密失败", err)
	}
	for _, input := range []string{"bad", cipher + "A"} {
		if _, err := enterpriseSecret(master, input, true); err == nil {
			t.Fatal("损坏密文被接受")
		}
	}
	if _, err := enterpriseSecret("short", "secret", false); err == nil {
		t.Fatal("弱主密钥被接受")
	}
	for _, url := range []string{"http://example.com", "https://user:pass@example.com", "https://example.com/#fragment"} {
		if _, err := enterpriseURL(url); err == nil {
			t.Fatal("危险 URL 被接受", url)
		}
	}
	for _, ip := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "100.64.0.1", "::1", "::ffff:192.168.1.1"} {
		if enterprisePublicIP(netip.MustParseAddr(ip)) {
			t.Fatal("危险 IP 被接受", ip)
		}
	}
	if !enterprisePublicIP(netip.MustParseAddr("8.8.8.8")) {
		t.Fatal("公网 IP 被拒绝")
	}
	body := &enterpriseBody{ReadCloser: io.NopCloser(strings.NewReader("1234")), remaining: 3}
	if _, err := io.ReadAll(body); err == nil {
		t.Fatal("超限响应被接受")
	}
	random, err := enterpriseRandom()
	if err != nil || len(random) != 43 {
		t.Fatal("随机 state 格式错误")
	}
}

// TestArchiveEncoding 验证确定性压缩、版本和内容哈希边界；前置白名单快照，无外部写入。
func TestArchiveEncoding(t *testing.T) {
	e := &iam.ArchiveEvent{ID: 1, Revision: 2, Payload: `{"request_id":"request","cost":0.1}`}
	a, hash, err := archiveEncode(e)
	if err != nil || len(hash) != 64 {
		t.Fatal(err)
	}
	b, other, err := archiveEncode(e)
	if err != nil || !bytes.Equal(a, b) || hash != other {
		t.Fatal("对象生成非确定性")
	}
	g, err := gzip.NewReader(bytes.NewReader(a))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(g)
	g.Close()
	if err != nil || !bytes.Contains(raw, []byte(`"event_id":1`)) {
		t.Fatalf("JSONL 格式错误 %s %v", raw, err)
	}
	if kodoContentHash(nil) != "Fto5o-5ea0sNMlW_75VgGJCv2AcJ" {
		t.Fatal("七牛空内容 etag 错误", kodoContentHash(nil))
	}
	if kodoContentHash(make([]byte, (4<<20)+1)) == kodoContentHash(make([]byte, 4<<20)) {
		t.Fatal("分块边界未区分")
	}
	if _, _, err := archiveEncode(&iam.ArchiveEvent{Payload: "bad"}); err == nil {
		t.Fatal("无效快照被接受")
	}
}

// TestArchiveCycleDue 验证轮次起点、精确边界和时钟回退；固定时间输入，无网络、持久化或清理。
func TestArchiveCycleDue(t *testing.T) {
	last := time.Unix(100000, 0)
	for _, tc := range []struct {
		elapsed time.Duration
		want    bool
	}{
		{0, false}, {-time.Second, false}, {5*time.Minute - time.Nanosecond, false}, {5 * time.Minute, true}, {6 * time.Minute, true},
	} {
		if got := archiveCycleDue(last.Add(tc.elapsed), last, 300); got != tc.want {
			t.Fatalf("归档周期判断失败 %v: %v", tc.elapsed, got)
		}
	}
}

// memoryArchive 为 regression 提供真实不可覆盖对象语义，只在测试注入，不改变生产适配器。
type memoryArchive struct {
	objects      map[string][]byte
	fail         bool
	failManifest bool
}

// Put 校验相同 key 的内容，参数为上下文和对象；返回注入失败或冲突，无网络调用。
func (m *memoryArchive) Put(ctx context.Context, key string, b []byte) error {
	if m.fail || (m.failManifest && strings.HasSuffix(key, ".manifest.json")) {
		return errors.New("offline")
	}
	if old, ok := m.objects[key]; ok && !bytes.Equal(old, b) {
		return errors.New("conflict")
	}
	m.objects[key] = append([]byte{}, b...)
	return ctx.Err()
}

// Delete 删除测试探针，参数为上下文/key，返回上下文错误；测试结束内存释放。
func (m *memoryArchive) Delete(ctx context.Context, key string) error {
	delete(m.objects, key)
	return ctx.Err()
}

// TestEnterpriseHTTP 验证真实路由、数据库补丁回滚、SSO 事务与归档数据面；前置隔离 stores。
// 覆盖凭据不回显、身份防合并、停用和租约 fencing；夹具删除 schema 与服务器。
func TestEnterpriseHTTP(t *testing.T) {
	cfg, st, db := testGatewayStores(t)
	cfg.GeneralSettings.MasterKey = strings.Repeat("m", 32)
	ctx := context.Background()
	admin, err := db.CreateUser(ctx, testActor, iam.UserInput{Email: "enterprise-admin@example.com", Password: "password123", Role: iam.RoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	org, err := db.CreateOrg(ctx, testActor, "Enterprise", nil)
	if err != nil {
		t.Fatal(err)
	}
	c, token, err := db.CreateDirectoryConnection(ctx, testActor, org.ID, "Enterprise")
	if err != nil {
		t.Fatal(err)
	}
	if err = db.ConfigureDirectory(ctx, testActor, c.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	du, u, err := db.ProvisionDirectoryUser(ctx, c.ID, "employee", "employee@example.com", "Employee", true)
	if err != nil {
		t.Fatal(err)
	}
	s := New(cfg, st, db)
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	session := loginAs(t, srv.URL, admin.Email, "password123")
	base := "/scim/v2/connections/" + c.ID
	for _, path := range []string{"/Schemas", "/Schemas/" + scimUserSchema, "/ResourceTypes", "/ResourceTypes/User", "/ServiceProviderConfig"} {
		status, b := authed(t, srv.URL, token, http.MethodGet, base+path, nil)
		if status != 200 {
			t.Fatalf("发现接口失败 %s: %d %s", path, status, b)
		}
	}
	patch := []byte(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"replace","path":"displayName","value":"Updated"}]}`)
	status, b := authed(t, srv.URL, token, http.MethodPatch, base+"/Users/"+du.ID, patch)
	if status != 200 || !bytes.Contains(b, []byte("Updated")) {
		t.Fatalf("用户 PATCH 失败 %d %s", status, b)
	}
	bad := []byte(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"replace","path":"displayName","value":"Rollback"},{"op":"replace","path":"role","value":"admin"}]}`)
	status, b = authed(t, srv.URL, token, http.MethodPatch, base+"/Users/"+du.ID, bad)
	if status != 400 {
		t.Fatalf("危险字段未拒绝 %d %s", status, b)
	}
	stored, err := db.DirectoryUserByID(ctx, c.ID, du.ID)
	if err != nil || stored.Name != "Updated" {
		t.Fatal("用户 PATCH 未回滚", err)
	}
	group, err := db.ReplaceDirectoryGroup(ctx, c.ID, "department", "Department", []string{du.ID})
	if err != nil {
		t.Fatal(err)
	}
	// 标准 JSON path 中引号是字面字符，不包含转义反斜线。
	gp, _ := json.Marshal(map[string]any{"schemas": []string{"urn:ietf:params:scim:api:messages:2.0:PatchOp"}, "Operations": []map[string]any{{"op": "remove", "path": fmtSCIMMemberPath(du.ID)}}})
	status, b = authed(t, srv.URL, token, http.MethodPatch, base+"/Groups/"+group.ID, gp)
	if status != 200 {
		t.Fatalf("组 PATCH 失败 %d %s", status, b)
	}
	configPath := "/admin/directory/connections/" + c.ID + "/oidc"
	oidcBody := []byte(`{"issuer":"https://identity.example.com","client_id":"xhub","client_secret":"secret-value","redirect_uri":"https://console.example.com/auth/enterprise/callback","return_uri":"https://console.example.com/ui/login","identity_claim":"sub","enabled":false}`)
	status, b = authed(t, srv.URL, session, http.MethodPut, configPath, oidcBody)
	if status != 200 {
		t.Fatalf("SSO 配置失败 %d %s", status, b)
	}
	status, b = authed(t, srv.URL, session, http.MethodGet, configPath, nil)
	if status != 200 || bytes.Contains(b, []byte("secret-value")) || bytes.Contains(b, []byte("secret")) {
		t.Fatalf("SSO 配置泄密 %d %s", status, b)
	}
	status, _ = authed(t, srv.URL, token, http.MethodGet, configPath, nil)
	if status == 200 {
		t.Fatal("SCIM token 可读取 SSO 配置")
	}
	oc, err := db.EnterpriseOIDCByID(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	oc.Enabled = true
	if err = db.SaveEnterpriseOIDC(ctx, testActor, *oc); err != nil {
		t.Fatal(err)
	}
	oc, err = db.EnterpriseOIDCByID(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	tx := iam.EnterpriseLoginTransaction{StateHash: iam.HashKey("state"), BrowserHash: iam.HashKey("browser"), ConnectionID: c.ID, Revision: oc.Revision, Nonce: "nonce", Verifier: "verifier", ExpiresAt: time.Now().Add(time.Minute)}
	if err = db.StartEnterpriseLogin(ctx, tx); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ConsumeEnterpriseLogin(ctx, "state", "wrong"); err == nil {
		t.Fatal("错误浏览器被接受")
	}
	if _, err = db.ConsumeEnterpriseLogin(ctx, "state", "browser"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ConsumeEnterpriseLogin(ctx, "state", "browser"); err == nil {
		t.Fatal("state 重放被接受")
	}
	if _, err = db.EnterpriseLoginUser(ctx, c.ID, oc.Issuer, "subject", "unknown", oc.Revision); err == nil {
		t.Fatal("未同步身份自动开户")
	}
	bound, err := db.EnterpriseLoginUser(ctx, c.ID, oc.Issuer, "subject", "employee", oc.Revision)
	if err != nil || bound.ID != u.ID {
		t.Fatal("稳定身份绑定失败", err)
	}
	if err = db.ConfigureDirectory(ctx, testActor, c.ID, false, ""); err != nil {
		t.Fatal(err)
	}
	options, err := db.EnterpriseOIDCOptions(ctx)
	if err != nil || len(options) != 1 {
		t.Fatal("停用 SCIM 不应隐藏独立 SSO 登录入口", options, err)
	}
	if err = db.StartEnterpriseLogin(ctx, tx); err != nil {
		t.Fatal("停用 SCIM 后独立 SSO 应能创建认证事务", err)
	}
	if _, err = db.EnterpriseLoginUser(ctx, c.ID, oc.Issuer, "subject", "employee", oc.Revision); err != nil {
		t.Fatal("停用 SCIM 不应阻止已同步稳定身份登录", err)
	}
	if err = db.ConfigureDirectory(ctx, testActor, c.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	archivePath := "/admin/archive/kodo"
	status, b = authed(t, srv.URL, session, http.MethodPut, archivePath, []byte(`{"bucket":"xhub-test","region":"z0","prefix":"billing","access_key":"fake-ak","secret_key":"fake-sk","enabled":true}`))
	if status != 200 {
		t.Fatalf("Kodo 配置失败 %d %s", status, b)
	}
	status, b = authed(t, srv.URL, session, http.MethodGet, archivePath, nil)
	if status != 200 || bytes.Contains(b, []byte("fake-ak")) || bytes.Contains(b, []byte("fake-sk")) {
		t.Fatalf("Kodo 配置泄密 %d %s", status, b)
	}
	// 旧客户端默认可回读；非法调度拒绝且不新增版本，显式调度保存后读取实际持久化值。
	var archiveSnapshot struct {
		Configuration iam.ArchiveConfig `json:"configuration"`
	}
	if err := json.Unmarshal(b, &archiveSnapshot); err != nil || archiveSnapshot.Configuration.IntervalSeconds != 2 || archiveSnapshot.Configuration.BatchSize != 20 {
		t.Fatal("归档默认调度未回读", string(b), err)
	}
	originalRevision := archiveSnapshot.Configuration.Revision
	for _, schedule := range []string{`"interval_seconds":1`, `"interval_seconds":86401`, `"batch_size":201`, `"batch_size":-1`} {
		status, b = authed(t, srv.URL, session, http.MethodPut, archivePath, []byte(`{"bucket":"xhub-test","region":"z0","prefix":"billing",`+schedule+`}`))
		if status != 400 || !bytes.Contains(b, []byte("invalid_request")) {
			t.Fatalf("非法归档调度被接受 %d %s", status, b)
		}
	}
	status, b = authed(t, srv.URL, session, http.MethodGet, archivePath, nil)
	if status != 200 || json.Unmarshal(b, &archiveSnapshot) != nil || archiveSnapshot.Configuration.Revision != originalRevision {
		t.Fatal("非法调度创建了配置版本", string(b))
	}
	status, b = authed(t, srv.URL, session, http.MethodPut, archivePath, []byte(`{"bucket":"xhub-test","region":"z0","prefix":"billing","interval_seconds":300,"batch_size":1,"enabled":true}`))
	if status != 200 {
		t.Fatalf("归档调度保存失败 %d %s", status, b)
	}
	status, b = authed(t, srv.URL, session, http.MethodGet, archivePath, nil)
	if status != 200 || json.Unmarshal(b, &archiveSnapshot) != nil || archiveSnapshot.Configuration.IntervalSeconds != 300 || archiveSnapshot.Configuration.BatchSize != 1 {
		t.Fatalf("归档调度持久化失败 %d %s", status, b)
	}
	record := iam.UsageRecord{RequestID: "archive-request", UserID: u.ID, OrganizationID: org.ID, Model: "test-model", Status: "success", RequestBody: "private prompt", ResponseBody: "private response"}
	if err = db.RecordUsage(ctx, []iam.UsageRecord{record, record}); err != nil {
		t.Fatal(err)
	}
	counts, _, err := db.ArchiveStatus(ctx)
	if err != nil || counts["pending"] != 1 {
		t.Fatalf("事务归档幂等失败 %v %v", counts, err)
	}
	// 非白名单展示别名变化不归档；真实账务结算变化必须保留独立事件版本。
	if _, err = db.Engine.Exec("UPDATE usage_events SET key_alias='changed alias' WHERE request_id=?", record.RequestID); err != nil {
		t.Fatal(err)
	}
	counts, _, err = db.ArchiveStatus(ctx)
	if err != nil || counts["pending"] != 1 {
		t.Fatal("别名更新重复归档", counts, err)
	}
	mem := &memoryArchive{objects: map[string][]byte{}, fail: true}
	factory := func(c *iam.ArchiveConfig) (archiveObjectStore, error) { return mem, nil }
	if count, err := s.runArchiveBatch(ctx, 20, factory); count != 1 || err == nil {
		t.Fatal("对象失败未反映")
	}
	counts, _, err = db.ArchiveStatus(ctx)
	if err != nil || counts["failed"] != 1 {
		t.Fatal("失败快照丢失", counts, err)
	}
	if err = db.RetryArchive(ctx, testActor); err != nil {
		t.Fatal(err)
	}
	mem.fail = false
	if count, err := s.runArchiveBatch(ctx, 1, factory); count != 1 || err != nil {
		t.Fatal("重试归档失败", err)
	}
	counts, events, err := db.ArchiveStatus(ctx)
	if err != nil || counts["done"] != 1 || len(mem.objects) != 2 || events[0].SHA256 == "" {
		t.Fatal("manifest 未确认", counts, events, err)
	}
	if count, err := s.runArchiveBatch(ctx, 20, factory); count != 0 || err != nil {
		t.Fatal("空队列仍上传对象", count, err)
	}
	for key, object := range mem.objects {
		if strings.HasSuffix(key, ".gz") {
			g, err := gzip.NewReader(bytes.NewReader(object))
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := io.ReadAll(g)
			g.Close()
			if bytes.Contains(raw, []byte("private")) {
				t.Fatal("归档泄露正文")
			}
		}
	}
	stale := events[0]
	stale.Lease = "stale"
	if ok, err := db.CompleteArchiveEvent(ctx, &stale, "bad", "bad", 1, true); ok || err != nil {
		t.Fatal("过期 worker 覆盖结果", err)
	}
	if _, err = db.Engine.Exec("UPDATE usage_events SET cost=cost+1,task_settled=true WHERE request_id=?", record.RequestID); err != nil {
		t.Fatal(err)
	}
	counts, _, err = db.ArchiveStatus(ctx)
	if err != nil || counts["pending"] != 1 || counts["done"] != 1 {
		t.Fatal("账务更新未保留新版本", counts, err)
	}
	record.RequestID = "archive-batch-second"
	if err := db.RecordUsage(ctx, []iam.UsageRecord{record}); err != nil {
		t.Fatal(err)
	}
	if count, err := s.runArchiveBatch(ctx, 1, factory); count != 1 || err != nil {
		t.Fatal("归档每轮上限执行失败", count, err)
	}
	counts, _, err = db.ArchiveStatus(ctx)
	if err != nil || counts["pending"] != 1 || counts["done"] != 2 {
		t.Fatal("归档超出每轮上限", counts, err)
	}
	original, err := db.ArchiveConfiguration(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	updated := *original
	updated.Prefix = "new-target"
	updated.Enabled = false
	if err = db.SaveArchiveConfiguration(ctx, testActor, updated); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.processArchive(ctx, func(target *iam.ArchiveConfig) (archiveObjectStore, error) {
		if target.Revision != original.Revision || target.Prefix != "billing" {
			t.Fatal("旧积压被改写到新目标", target)
		}
		return mem, nil
	}); !ok || err != nil {
		t.Fatal("旧版本积压无法处理", err)
	}
	status, b = authed(t, srv.URL, session, http.MethodPut, archivePath, []byte(`{"bucket":"xhub-test","region":"invalid","prefix":"billing"}`))
	if status != 400 {
		t.Fatalf("不支持区域被接受 %d %s", status, b)
	}
	if _, _, err = db.ProvisionDirectoryUser(ctx, c.ID, "employee", "employee@example.com", "Employee", false); err != nil {
		t.Fatal(err)
	}
	if _, err = db.EnterpriseLoginUser(ctx, c.ID, oc.Issuer, "subject", "employee", oc.Revision); err == nil {
		t.Fatal("停用身份仍可登录")
	}
}

// fmtSCIMMemberPath 生成精确成员筛选路径；参数为受控 id，返回协议路径，无副作用。
func fmtSCIMMemberPath(id string) string {
	encoded, _ := json.Marshal(id)
	return "members[value eq " + string(encoded) + "]"
}
