package models

import (
	"context"
	"reflect"
	"testing"

	"github.com/sunqirui1987/xhub/cmd/regression/testsupport"
	"github.com/sunqirui1987/xhub/internal/auth"
	"github.com/sunqirui1987/xhub/internal/authz"
	"github.com/sunqirui1987/xhub/internal/iam"
)

// TestEffectiveOrganizationModels 验证会话模型发现与本人组织交集一致；前置隔离数据库、两个组织及唯一成员账号。
// 参数 t 为测试上下文，无返回；覆盖缺省/伪造团队、平台预览和存储错误拒绝，夹具删除 schema 并关闭连接。
func TestEffectiveOrganizationModels(t *testing.T) {
	ctx := context.Background()
	db, err := iam.Open(ctx, testsupport.Postgres(t, "model_ceiling"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	h := &templateGroupTestHost{db: db}
	by := iam.Actor{}
	org, err := db.CreateOrgWithConfig(ctx, by, "mine", nil, iam.OrgConfig{Models: []string{"a"}})
	if err != nil {
		t.Fatal(err)
	}
	team, err := db.CreateTeam(ctx, by, iam.TeamInput{OrganizationID: org.ID, Name: "mine"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := db.CreateOrgWithConfig(ctx, by, "other", nil, iam.OrgConfig{Models: []string{"b"}})
	if err != nil {
		t.Fatal(err)
	}
	otherTeam, err := db.CreateTeam(ctx, by, iam.TeamInput{OrganizationID: other.ID, Name: "other"})
	if err != nil {
		t.Fatal(err)
	}
	user, err := db.CreateUser(ctx, by, iam.UserInput{Email: "member@example.com", Password: "test-password"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.AddMember(ctx, by, team.ID, user.Email, iam.TeamAdmin); err != nil {
		t.Fatal(err)
	}
	p := &auth.Principal{Kind: authz.KindSession, Role: iam.RoleUser, UserID: user.ID}
	for _, teamID := range []string{"", team.ID, otherTeam.ID} {
		models, err := effectiveModels(h, ctx, p, teamID)
		if err != nil || !reflect.DeepEqual(models, []string{"a"}) {
			t.Fatalf("会话发现绕过本人组织上限: %v %v", models, err)
		}
	}
	p.Role = iam.RoleAdmin
	models, err := effectiveModels(h, ctx, p, otherTeam.ID)
	if err != nil || !reflect.DeepEqual(models, []string{"b"}) {
		t.Fatalf("平台预览错误: %v %v", models, err)
	}
	p.Role = iam.RoleUser
	if !AllowsModel(h, ctx, p, "", "a") || AllowsModel(h, ctx, p, "", "b") || AllowsModel(h, ctx, p, "", "") {
		t.Fatal("模型授权决定错误")
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	if AllowsModel(h, ctx, p, "", "a") {
		t.Fatal("数据库失败时不应放行")
	}
}
