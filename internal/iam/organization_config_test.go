package iam

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
)

// TestNormalizeOrganizationModels 验证模型范围归一化；前置内存输入，覆盖空值、去重、特殊项和非法名称。
// 参数 t 为测试上下文，无返回；确认不修改输入，无持久化数据，不需清理。
func TestNormalizeOrganizationModels(t *testing.T) {
	for _, tc := range []struct {
		name     string
		in, want []string
		invalid  bool
	}{
		{name: "empty", want: []string{}}, {name: "deduplicate", in: []string{"b", "a", "a"}, want: []string{"a", "b"}},
		{name: "all", in: []string{"all-proxy-models"}, want: []string{}},
		{name: "none", in: []string{"no-default-models"}, want: []string{"no-default-models"}},
		{name: "blank", in: []string{""}, invalid: true}, {name: "space", in: []string{" a"}, invalid: true},
		{name: "mixed-all", in: []string{"all-proxy-models", "a"}, invalid: true},
		{name: "mixed-none", in: []string{"no-default-models", "a"}, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := append([]string(nil), tc.in...)
			got, err := NormalizeOrganizationModels(tc.in)
			if tc.invalid {
				if !errors.Is(err, ErrInvalid) {
					t.Fatalf("非法范围未拒绝: %v", err)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("范围归一化错误: %v %v", got, err)
			}
			if len(got) > 0 {
				got[0] = "changed"
			}
			if !reflect.DeepEqual(tc.in, original) {
				t.Fatal("修改返回值影响了原输入")
			}
		})
	}
}

// TestNormalizeOrganizationMetadata 验证元数据对象深拷贝；前置嵌套 Unicode 输入，验证空对象和非 JSON 失败路径。
// 参数 t 为测试上下文，无返回；不写数据，测试结束无需清理。
func TestNormalizeOrganizationMetadata(t *testing.T) {
	in := map[string]any{"nested": map[string]any{"department": "研发"}}
	got, err := normalizeOrganizationMetadata(in)
	if err != nil || !reflect.DeepEqual(got, in) {
		t.Fatalf("元数据归一化失败: %v %v", got, err)
	}
	got["nested"].(map[string]any)["department"] = "changed"
	if in["nested"].(map[string]any)["department"] != "研发" {
		t.Fatal("元数据未深拷贝")
	}
	empty, err := normalizeOrganizationMetadata(nil)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("空元数据错误: %v %v", empty, err)
	}
	for _, value := range []any{math.NaN(), math.Inf(1), func() {}} {
		if _, err := normalizeOrganizationMetadata(map[string]any{"invalid": value}); !errors.Is(err, ErrInvalid) {
			t.Fatalf("非 JSON 对象未拒绝: %v", err)
		}
	}
}

// TestOrganizationConfigStorage 验证创建、更新、拒绝回滚和清空；前置隔离 PostgreSQL，核对真实落库。
// 参数 t 为测试上下文，无返回；testDB 自动删除专属 schema，不使用外部模型。
func TestOrganizationConfigStorage(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	by := Actor{Kind: "master"}
	org, err := db.CreateOrgWithConfig(ctx, by, "configured", nil, OrgConfig{Models: []string{"a"}, Metadata: map[string]any{"department": "研发"}})
	if err != nil {
		t.Fatal(err)
	}
	name := "renamed"
	updated, err := db.UpdateOrg(ctx, by, org.ID, OrgUpdate{Name: &name})
	if err != nil || !reflect.DeepEqual(updated.Models, org.Models) || !reflect.DeepEqual(updated.Metadata, org.Metadata) {
		t.Fatalf("部分更新丢失配置: %+v %v", updated, err)
	}
	bad := []string{""}
	if _, err := db.UpdateOrg(ctx, by, org.ID, OrgUpdate{Name: &name, Models: &bad}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("错误输入未拒绝: %v", err)
	}
	badMeta := map[string]any{"bad": math.NaN()}
	if _, err := db.UpdateOrg(ctx, by, org.ID, OrgUpdate{Metadata: &badMeta}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("非 JSON 更新未拒绝: %v", err)
	}
	current, err := db.GetOrg(ctx, org.ID)
	if err != nil || !reflect.DeepEqual(current.Models, org.Models) || !reflect.DeepEqual(current.Metadata, org.Metadata) {
		t.Fatalf("失败更新破坏原数据: %+v %v", current, err)
	}
	empty := []string{}
	var meta map[string]any
	cleared, err := db.UpdateOrg(ctx, by, org.ID, OrgUpdate{Models: &empty, Metadata: &meta})
	if err != nil || cleared.Models == nil || len(cleared.Models) != 0 || cleared.Metadata == nil || len(cleared.Metadata) != 0 {
		t.Fatalf("清空失败: %+v %v", cleared, err)
	}
	for _, cfg := range []OrgConfig{{Models: bad}, {Metadata: badMeta}} {
		if _, err := db.CreateOrgWithConfig(ctx, by, "invalid", nil, cfg); !errors.Is(err, ErrInvalid) {
			t.Fatalf("非法创建未拒绝: %v", err)
		}
	}
	rows, err := db.ListOrgs(ctx, "")
	if err != nil || len(rows) != 1 {
		t.Fatalf("非法创建留下组织: %v %v", rows, err)
	}
	if _, err := db.UpdateOrg(ctx, by, "missing", OrgUpdate{Models: &empty}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("不存在组织错误: %v", err)
	}
}

// TestOrganizationConfigUpgrade 验证现有组织表升级幂等且不改变名称或消费；前置隔离 PostgreSQL 的旧列结构。
// 参数 t 为测试上下文，无返回；两次迁移补齐空模型和空元数据，testDB 清理整个 schema。
func TestOrganizationConfigUpgrade(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	org, err := db.CreateOrg(ctx, Actor{}, "legacy", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Engine.Exec("UPDATE organizations SET spend = 1.25 WHERE id = ?", org.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Engine.Exec("ALTER TABLE organizations DROP COLUMN models, DROP COLUMN metadata"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = db.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	got, err := db.GetOrg(ctx, org.ID)
	if err != nil || got.Name != "legacy" || got.Spend != 1.25 || got.Models == nil || len(got.Models) != 0 || got.Metadata == nil || len(got.Metadata) != 0 {
		t.Fatalf("迁移丢失原数据或默认值错误: %+v %v", got, err)
	}
}

// TestOrganizationModelHierarchy 验证团队创建、修改、转移的上限校验及运行期交集；前置独立组织和团队。
// 参数 t 为测试上下文，无返回；覆盖继承、非法越界、收窄后无交集和清空恢复，testDB 删除专属 schema。
func TestOrganizationModelHierarchy(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	by := Actor{}
	org, err := db.CreateOrgWithConfig(ctx, by, "limited", nil, OrgConfig{Models: []string{"a", "b"}})
	if err != nil {
		t.Fatal(err)
	}
	other, err := db.CreateOrgWithConfig(ctx, by, "other", nil, OrgConfig{Models: []string{"c"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.CreateTeam(ctx, by, TeamInput{OrganizationID: org.ID, Name: "invalid", Models: []string{"c"}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("越界创建团队: %v", err)
	}
	team, err := db.CreateTeam(ctx, by, TeamInput{OrganizationID: org.ID, Name: "valid", Models: []string{"a"}})
	if err != nil {
		t.Fatal(err)
	}
	invalid := []string{"c"}
	if _, err = db.AdminUpdateTeam(ctx, by, team.ID, nil, nil, &invalid); !errors.Is(err, ErrInvalid) {
		t.Fatalf("越界更新团队: %v", err)
	}
	if _, err = db.MoveTeam(ctx, by, team.ID, other.ID); !errors.Is(err, ErrInvalid) {
		t.Fatalf("越界移动团队: %v", err)
	}
	current, err := db.GetTeam(ctx, team.ID)
	if err != nil || current.OrganizationID != org.ID || !reflect.DeepEqual(current.Models, []string{"a"}) {
		t.Fatalf("拒绝操作改变团队: %+v %v", current, err)
	}
	models, err := db.AllowedModelsForTeam(ctx, team.ID)
	if err != nil || !reflect.DeepEqual(models, []string{"a"}) {
		t.Fatalf("团队交集错误: %v %v", models, err)
	}
	narrowed := []string{"b"}
	if _, err = db.UpdateOrg(ctx, by, org.ID, OrgUpdate{Models: &narrowed}); err != nil {
		t.Fatal(err)
	}
	models, err = db.AllowedModelsForTeam(ctx, team.ID)
	if err != nil || models == nil || len(models) != 0 {
		t.Fatalf("空交集必须拒绝全部而非不限制: %v %v", models, err)
	}
	empty := []string{}
	if _, err = db.AdminUpdateTeam(ctx, by, team.ID, nil, nil, &empty); err != nil {
		t.Fatal(err)
	}
	models, err = db.AllowedModelsForTeam(ctx, team.ID)
	if err != nil || !reflect.DeepEqual(models, []string{"b"}) {
		t.Fatalf("空团队范围未继承组织: %v %v", models, err)
	}
	if _, err = db.MoveTeam(ctx, by, team.ID, other.ID); err != nil {
		t.Fatal(err)
	}
	models, err = db.AllowedModelsForTeam(ctx, team.ID)
	if err != nil || !reflect.DeepEqual(models, []string{"c"}) {
		t.Fatalf("移动后未使用目标组织上限: %v %v", models, err)
	}
	if _, err = db.AllowedModelsForTeam(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("不存在团队不应放开权限: %v", err)
	}
}
