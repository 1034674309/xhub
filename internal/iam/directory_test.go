package iam

import (
	"context"
	"errors"
	"testing"
)

// TestDirectoryLifecycle 验证真实目录数据库生命周期；前置隔离 schema、组织和团队。
// 验证凭据范围、幂等用户、多组稳定分配、人工覆盖、移除迁移、停用与失败回滚；testDB 清理全部数据。
func TestDirectoryLifecycle(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	by := Actor{Kind: "master"}
	org, err := db.CreateOrg(ctx, by, "Directory company", nil)
	if err != nil {
		t.Fatal(err)
	}
	first, err := db.CreateTeam(ctx, by, TeamInput{OrganizationID: org.ID, Name: "Engineering"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := db.CreateTeam(ctx, by, TeamInput{OrganizationID: org.ID, Name: "Office"})
	if err != nil {
		t.Fatal(err)
	}
	connection, token, err := db.CreateDirectoryConnection(ctx, by, org.ID, "Corporate directory")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.AuthenticateDirectory(ctx, connection.ID, token); !errors.Is(err, ErrInactive) {
		t.Fatal("未启用连接可认证")
	}
	if err := db.ConfigureDirectory(ctx, by, connection.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := db.AuthenticateDirectory(ctx, connection.ID, token); err != nil {
		t.Fatal("有效同步凭据被拒绝", err)
	}
	if _, err := db.AuthenticateDirectory(ctx, connection.ID, token+"bad"); !errors.Is(err, ErrInactive) {
		t.Fatal("错误凭据被接受")
	}
	identity, user, err := db.ProvisionDirectoryUser(ctx, connection.ID, "employee-1", "member@example.com", "Member", true)
	if err != nil {
		t.Fatal(err)
	}
	if user.Role != RoleUser || user.MaxBudget == nil || *user.MaxBudget != 0 || user.PasswordHash != "" {
		t.Fatal("目录用户获得默认权限或密码")
	}
	same, _, err := db.ProvisionDirectoryUser(ctx, connection.ID, "employee-1", "member@example.com", "Member updated", true)
	if err != nil || same.ID != identity.ID {
		t.Fatal("重复同步没有保持稳定身份", err)
	}
	department, err := db.ReplaceDirectoryGroup(ctx, connection.ID, "department", "Department", []string{identity.ID})
	if err != nil {
		t.Fatal(err)
	}
	office, err := db.ReplaceDirectoryGroup(ctx, connection.ID, "office", "Office", []string{identity.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SetDirectoryMapping(ctx, by, DirectoryMapping{ConnectionID: connection.ID, GroupID: department.ID, TeamID: first.ID, Priority: 1, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := db.SetDirectoryMapping(ctx, by, DirectoryMapping{ConnectionID: connection.ID, GroupID: office.ID, TeamID: second.ID, Priority: 0, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	assertDirectoryTeam(t, db, user.ID, first.ID)
	snapshot, err := db.DirectoryManagementSnapshot(ctx, connection.ID)
	if err != nil || len(snapshot.Users) != 1 || snapshot.Users[0].TeamID != first.ID || len(snapshot.Groups) != 2 || len(snapshot.Mappings) != 2 {
		t.Fatalf("管理快照归属错误: %+v %v", snapshot, err)
	}
	if _, err := db.DirectoryManagementSnapshot(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("不存在连接未拒绝: %v", err)
	}
	// 另一个用户恢复自动时，不能让现有有效团队因新优先级迁移。
	peer, peerUser, err := db.ProvisionDirectoryUser(ctx, connection.ID, "employee-peer", "peer@example.com", "Peer", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ReplaceDirectoryGroup(ctx, connection.ID, "department", "Department", []string{identity.ID, peer.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ReplaceDirectoryGroup(ctx, connection.ID, "office", "Office", []string{identity.ID, peer.ID}); err != nil {
		t.Fatal(err)
	}
	if err := db.OverrideDirectoryTeam(ctx, by, peerUser.ID, "auto", ""); err != nil {
		t.Fatal(err)
	}
	assertDirectoryTeam(t, db, peerUser.ID, second.ID)
	assertDirectoryTeam(t, db, user.ID, first.ID)
	if err := db.OverrideDirectoryTeam(ctx, by, user.ID, "manual", second.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ReplaceDirectoryGroup(ctx, connection.ID, "office", "Office", nil); err != nil {
		t.Fatal(err)
	}
	assertDirectoryTeam(t, db, user.ID, second.ID)
	if err := db.OverrideDirectoryTeam(ctx, by, user.ID, "manual", ""); err != nil {
		t.Fatal(err)
	}
	assertDirectoryTeam(t, db, user.ID, "")
	if err := db.OverrideDirectoryTeam(ctx, by, user.ID, "auto", ""); err != nil {
		t.Fatal(err)
	}
	assertDirectoryTeam(t, db, user.ID, first.ID)
	if _, err := db.ReplaceDirectoryGroup(ctx, connection.ID, "department", "Must roll back", []string{"missing"}); err == nil {
		t.Fatal("未知组成员未拒绝")
	}
	assertDirectoryTeam(t, db, user.ID, first.ID)
	var stored DirectoryGroup
	if _, err := db.Engine.ID(department.ID).Get(&stored); err != nil || stored.Name != "Department" {
		t.Fatal("失败组更新没有回滚", err)
	}
	if _, err := db.ReplaceDirectoryGroup(ctx, connection.ID, "department", "Department", nil); err != nil {
		t.Fatal(err)
	}
	assertDirectoryTeam(t, db, user.ID, "")
	if _, _, err := db.ProvisionDirectoryUser(ctx, connection.ID, "employee-other", user.Email, "Collision", true); !errors.Is(err, ErrConflict) {
		t.Fatal("同邮箱被合并", err)
	}
	if _, _, err := db.ProvisionDirectoryUser(ctx, connection.ID, identity.ExternalID, user.Email, "Member", false); err != nil {
		t.Fatal(err)
	}
	disabled, err := db.GetUser(ctx, user.ID)
	if err != nil || disabled.Active() || disabled.SessionVersion != user.SessionVersion+1 {
		t.Fatalf("目录停用未撤销会话: %+v %v", disabled, err)
	}
	other, err := db.CreateOrg(ctx, by, "Other company", nil)
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := db.CreateTeam(ctx, by, TeamInput{OrganizationID: other.ID, Name: "Foreign"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.OverrideDirectoryTeam(ctx, by, user.ID, "manual", foreign.ID); !errors.Is(err, ErrInvalid) {
		t.Fatal("允许跨组织人工覆盖", err)
	}
	if err := db.SetDirectoryMapping(ctx, by, DirectoryMapping{ConnectionID: connection.ID, GroupID: department.ID, TeamID: foreign.ID, Enabled: true}); !errors.Is(err, ErrInvalid) {
		t.Fatal("允许跨组织映射", err)
	}
}

// assertDirectoryTeam 读取真实成员关系验证单团队；参数为测试、隔离库、用户及期望团队。
// 无返回或副作用；查询失败或成员数/团队不符时终止测试，数据由调用方夹具清理。
func assertDirectoryTeam(t *testing.T, db *DB, userID, expected string) {
	t.Helper()
	var rows []TeamMembership
	if err := db.Engine.Where("user_id = ?", userID).Find(&rows); err != nil {
		t.Fatal(err)
	}
	if expected == "" {
		if len(rows) != 0 {
			t.Fatalf("应无团队: %+v", rows)
		}
		return
	}
	if len(rows) != 1 || rows[0].TeamID != expected {
		t.Fatalf("单团队归属不符，期望 %s: %+v", expected, rows)
	}
}
