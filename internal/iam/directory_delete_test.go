package iam

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestDirectoryDelete 前置隔离数据库及含用户、组、映射、SSO 身份与登录事务的连接。
// 验证不存在、审计失败原子回滚、成功清理及凭据撤销、本地用户和归属保留、同组织重建；testDB 清理 schema。
func TestDirectoryDelete(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	by := Actor{Kind: "master"}
	org, err := db.CreateOrg(ctx, by, "Delete directory company", nil)
	if err != nil {
		t.Fatal(err)
	}
	team, err := db.CreateTeam(ctx, by, TeamInput{OrganizationID: org.ID, Name: "Kept team"})
	if err != nil {
		t.Fatal(err)
	}
	c, token, err := db.CreateDirectoryConnection(ctx, by, org.ID, "Delete directory")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.ConfigureDirectory(ctx, by, c.ID, true, team.ID); err != nil {
		t.Fatal(err)
	}
	du, user, err := db.ProvisionDirectoryUser(ctx, c.ID, "stable-delete", "delete-member@example.com", "Kept user", true)
	if err != nil {
		t.Fatal(err)
	}
	group, err := db.ReplaceDirectoryGroup(ctx, c.ID, "delete-group", "Kept membership", []string{du.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SetDirectoryMapping(ctx, by, DirectoryMapping{ConnectionID: c.ID, GroupID: group.ID, TeamID: team.ID, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	cfg := EnterpriseOIDC{Name: "Independent login", ConnectionID: c.ID, Issuer: "https://identity.example.com", ClientID: "client", RedirectURI: "https://console.example.com/auth/enterprise/callback", ReturnURI: "https://console.example.com/ui/login", IdentityClaim: "sub", Enabled: true}
	if _, err := db.CreateEnterpriseOIDC(ctx, by, org.ID, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := db.EnterpriseLoginUser(ctx, c.ID, cfg.Issuer, "subject-delete", du.ExternalID, 1); err != nil {
		t.Fatal(err)
	}
	if err := db.StartEnterpriseLogin(ctx, EnterpriseLoginTransaction{StateHash: HashKey("state-delete"), BrowserHash: HashKey("browser-delete"), ConnectionID: c.ID, Revision: 1, Nonce: "nonce", Verifier: "verifier", ExpiresAt: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteDirectoryConnection(ctx, by, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("不存在连接删除错误: %v", err)
	}
	// 用数据库约束在最终审计阶段失败，证明已经执行的目录清理也完整回滚。
	if _, err := db.Engine.Exec("ALTER TABLE audit_logs ADD CONSTRAINT reject_directory_delete CHECK (action <> 'directory.delete')"); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteDirectoryConnection(ctx, by, c.ID); err == nil {
		t.Fatal("审计失败未回滚")
	}
	if _, err := db.AuthenticateDirectory(ctx, c.ID, token); err != nil {
		t.Fatalf("失败删除撤销凭据: %v", err)
	}
	if _, err := db.Engine.Exec("ALTER TABLE audit_logs DROP CONSTRAINT reject_directory_delete"); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteDirectoryConnection(ctx, by, c.ID); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"directory_team_mappings", "directory_group_members", "directory_groups"} {
		rows, err := db.Engine.QueryString("SELECT count(*) AS n FROM "+table+" WHERE connection_id = ?", c.ID)
		if err != nil || len(rows) != 1 || rows[0]["n"] != "0" {
			t.Fatalf("目录子表未清理 %s: %+v %v", table, rows, err)
		}
	}
	if _, err := db.AuthenticateDirectory(ctx, c.ID, token); !errors.Is(err, ErrInactive) {
		t.Fatalf("旧凭据仍有效: %v", err)
	}
	if _, err := db.ConsumeEnterpriseLogin(ctx, "state-delete", "browser-delete"); err != nil {
		t.Fatalf("删除 SCIM 意外撤销独立 SSO 事务: %v", err)
	}
	for _, table := range []string{"enterprise_login_identities", "enterprise_oidc", "directory_users"} {
		rows, err := db.Engine.QueryString("SELECT count(*) AS n FROM "+table+" WHERE connection_id=?", c.ID)
		if err != nil || rows[0]["n"] != "1" {
			t.Fatalf("SCIM 删除误删稳定身份或 SSO %s: %+v %v", table, rows, err)
		}
	}
	kept, err := db.GetUser(ctx, user.ID)
	if err != nil || kept.Status != StatusActive || kept.SessionVersion != user.SessionVersion {
		t.Fatalf("本地用户被改变: %+v %v", kept, err)
	}
	assertDirectoryTeam(t, db, user.ID, team.ID)
	if err := db.DeleteDirectoryConnection(ctx, by, c.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("重复删除错误: %v", err)
	}
	replacement, newToken, err := db.CreateDirectoryConnection(ctx, by, org.ID, "Reconnected")
	if err != nil || replacement.ID != c.ID || newToken == token {
		t.Fatalf("同组织无法安全重建: %+v %v", replacement, err)
	}
}
