package iam

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestEnterpriseIndependent 前置隔离数据库，验证无 SCIM 创建 SSO、独立名称/开关及双向删除隔离。
// 同时覆盖重复、空名称、缺失组织、审计失败回滚、删除后禁止同步管理；testDB 清理全部数据。
func TestEnterpriseIndependent(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	by := Actor{Kind: "master"}
	org, err := db.CreateOrg(ctx, by, "Independent company", nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := EnterpriseOIDC{Name: "Login name", Issuer: "https://identity.example.com", ClientID: "client", IdentityClaim: "sub", Enabled: true}
	if _, err := db.CreateEnterpriseOIDC(ctx, by, org.ID, EnterpriseOIDC{Name: "   "}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("空名称未拒绝: %v", err)
	}
	if _, err := db.CreateEnterpriseOIDC(ctx, by, "missing", cfg); !errors.Is(err, ErrNotFound) {
		t.Fatalf("缺失组织未拒绝: %v", err)
	}
	sso, err := db.CreateEnterpriseOIDC(ctx, by, org.ID, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if rows, err := db.DirectoryConnections(ctx); err != nil || len(rows) != 0 {
		t.Fatalf("SSO 意外创建 SCIM 配置: %+v %v", rows, err)
	}
	if _, err := db.CreateEnterpriseOIDC(ctx, by, org.ID, cfg); !errors.Is(err, ErrConflict) {
		t.Fatalf("重复 SSO 未拒绝: %v", err)
	}
	if rows, err := db.EnterpriseOIDCOptions(ctx); err != nil || len(rows) != 1 {
		t.Fatalf("独立 SSO 入口不可用: %+v %v", rows, err)
	}
	tx := EnterpriseLoginTransaction{ConnectionID: sso.ConnectionID, Revision: 1, StateHash: HashKey("independent"), BrowserHash: HashKey("browser"), ExpiresAt: time.Now().Add(time.Minute)}
	if err := db.StartEnterpriseLogin(ctx, tx); err != nil {
		t.Fatalf("无 SCIM 不能开始 SSO: %v", err)
	}
	c, token, err := db.CreateDirectoryConnection(ctx, by, org.ID, "Sync name")
	if err != nil || c.ID != sso.ConnectionID {
		t.Fatalf("SCIM 未复用组织身份锚点: %+v %v", c, err)
	}
	stored, err := db.EnterpriseOIDCByID(ctx, c.ID)
	if err != nil || stored.Name != cfg.Name || !stored.Enabled {
		t.Fatalf("SCIM 创建覆盖 SSO: %+v %v", stored, err)
	}
	stored.Name = "   "
	if err := db.SaveEnterpriseOIDC(ctx, by, *stored); !errors.Is(err, ErrInvalid) {
		t.Fatalf("空白 SSO 名称未拒绝: %v", err)
	}
	stored.Name = ""
	if err := db.SaveEnterpriseOIDC(ctx, by, *stored); !errors.Is(err, ErrInvalid) {
		t.Fatalf("省略 SSO 名称未拒绝: %v", err)
	}
	if saved, err := db.EnterpriseOIDCByID(ctx, c.ID); err != nil || saved.Name != cfg.Name {
		t.Fatalf("非法更新改变独立 SSO 名称: %+v %v", saved, err)
	}
	if err := db.ConfigureDirectory(ctx, by, c.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Engine.Exec("ALTER TABLE audit_logs ADD CONSTRAINT reject_sso_delete CHECK(action <> 'enterprise.oidc.delete')"); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteEnterpriseOIDC(ctx, by, c.ID); err == nil {
		t.Fatal("SSO 审计失败未回滚")
	}
	if _, err := db.EnterpriseOIDCByID(ctx, c.ID); err != nil {
		t.Fatal("回滚丢失 SSO", err)
	}
	if _, err := db.Engine.Exec("ALTER TABLE audit_logs DROP CONSTRAINT reject_sso_delete"); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteEnterpriseOIDC(ctx, by, c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.AuthenticateDirectory(ctx, c.ID, token); err != nil {
		t.Fatalf("SSO 删除影响 SCIM: %v", err)
	}
	if _, err := db.ConsumeEnterpriseLogin(ctx, "independent", "browser"); err == nil {
		t.Fatal("SSO 删除未撤销登录事务")
	}
	if err := db.DeleteEnterpriseOIDC(ctx, by, c.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("重复删除未拒绝: %v", err)
	}
	cfg.ConnectionID = c.ID
	if err := db.SaveEnterpriseOIDC(ctx, by, cfg); !errors.Is(err, ErrNotFound) {
		t.Fatalf("更新意外重建已删除 SSO: %v", err)
	}
	if _, err := db.CreateEnterpriseOIDC(ctx, by, org.ID, cfg); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteDirectoryConnection(ctx, by, c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.EnterpriseOIDCByID(ctx, c.ID); err != nil {
		t.Fatalf("SCIM 删除影响 SSO: %v", err)
	}
	if _, err := db.DirectoryManagementSnapshot(ctx, c.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("已删除 SCIM 仍可管理: %v", err)
	}
	if err := db.RotateDirectoryToken(ctx, by, c.ID, HashKey("new")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("已删除 SCIM 可轮换凭据: %v", err)
	}
	if _, _, err := db.ProvisionDirectoryUser(ctx, c.ID, "subject", "member@example.com", "Member", true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("已删除 SCIM 可同步用户: %v", err)
	}
}
