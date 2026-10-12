package models

import (
	"context"
	"github.com/sunqirui1987/xhub/cmd/regression/testsupport"
	"github.com/sunqirui1987/xhub/internal/config"
	"github.com/sunqirui1987/xhub/internal/iam"
	"strings"
	"testing"
)

// TestTemplateGroupsWithoutIdentity 验证无身份库时模型管理没有模板引用，且不读取独立组配置。
// 前置内存宿主；名称和最后部署检查不修改目录，测试不创建外部数据，无需清理。
func TestTemplateGroupsWithoutIdentity(t *testing.T) {
	h := &modelTestHost{models: []config.ModelEntry{{ModelName: "a"}}}
	groups, err := templateGroups(h)
	if err != nil || len(groups) != 0 {
		t.Fatalf("无身份库产生模板组: %v %v", groups, err)
	}
	if err := checkRoutingGroupName(h, "a"); err != nil {
		t.Fatal(err)
	}
	if err := checkRoutingGroupRemoval(h, "a"); err != nil {
		t.Fatal(err)
	}
	if len(h.models) != 1 {
		t.Fatal("引用检查修改目录")
	}
}

// templateGroupTestHost 在内存模型宿主上提供隔离身份库，仅供模板引用保护测试使用。
type templateGroupTestHost struct {
	// 复用模型目录与锁实现，不连接实际供应商。
	modelTestHost
	// 当前测试私有 schema 中的真实模板存储，测试结束关闭并销毁。
	db *iam.DB
}

// Identity 返回测试专属身份库，无参数、无副作用；模型引用检查通过它读取真实模板。
func (h *templateGroupTestHost) Identity() *iam.DB { return h.db }

// TestTemplateGroupsStoredBody 验证模型改名和删除读取全部模板，并在存储正文损坏时拒绝操作。
// 前置真实隔离 PostgreSQL；覆盖正常组、空配置、非法 JSON、null、数组和非法规则。
// 校验保护结果且模型目录不变；每例删除模板，测试结束关闭连接并由夹具清理 schema。
func TestTemplateGroupsStoredBody(t *testing.T) {
	ctx := context.Background()
	db, err := iam.Open(ctx, testsupport.Postgres(t, "model_template_groups"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	h := &templateGroupTestHost{modelTestHost: modelTestHost{models: []config.ModelEntry{{ModelName: "member"}}}, db: db}
	for _, tc := range []struct {
		// 子用例名称，标明正文边界。
		name string
		// 直接写入存储的正文，故意绕过管理 API 校验来模拟损坏。
		body string
		// 是否应拒绝解析，防止损坏正文被当作空引用。
		wantError bool
		// 正常正文预期包含的模板组数。
		groups int
	}{
		{"group", `{"routing_groups":[{"group_name":"pool","models":["member"],"routing_strategy":"traffic-split"}]}`, false, 1},
		{"empty", `{}`, false, 0},
		{"invalid_json", `{`, true, 0},
		{"null", `null`, true, 0},
		{"array", `[]`, true, 0},
		{"invalid_rule", `{"routing_groups":[{"group_name":"pool","models":[]}]}`, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row, err := db.CreateRouteTemplate(ctx, iam.Actor{}, iam.TemplateOwner{}, tc.name, tc.body)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := db.DeleteRouteTemplate(ctx, iam.Actor{}, row.ID); err != nil {
					t.Errorf("清理模板失败: %v", err)
				}
			})
			groups, err := templateGroups(h)
			if (err != nil) != tc.wantError || len(groups) != tc.groups {
				t.Fatalf("模板解析错误: groups=%v err=%v", groups, err)
			}
			nameErr := checkRoutingGroupName(h, "pool")
			removeErr := checkRoutingGroupRemoval(h, "member")
			blocked := tc.wantError || tc.groups > 0
			if (nameErr != nil) != blocked || (removeErr != nil) != blocked {
				t.Fatalf("模板引用保护错误: name=%v remove=%v", nameErr, removeErr)
			}
			if tc.groups > 0 && (!strings.Contains(nameErr.Error(), "shadows") || !strings.Contains(removeErr.Error(), "remove model")) {
				t.Fatalf("正常引用未返回具体冲突: %v / %v", nameErr, removeErr)
			}
			if len(h.models) != 1 || h.models[0].ModelName != "member" {
				t.Fatal("引用检查修改了模型目录")
			}
		})
	}
}
