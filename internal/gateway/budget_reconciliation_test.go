package gateway

import (
	"net/http"
	"strings"
	"testing"

	"github.com/sunqirui1987/xhub/internal/iam"
)

// TestBudgetReconciliationManagementGate 验证组织/团队管理员、普通用户、管理员虚拟Key和master都不能核对费用。
// 前置私有PG真实网关，管理员session可读空列表；所有禁止写操作返回403且没有账务副作用，fixture自动清理。
func TestBudgetReconciliationManagementGate(t *testing.T) {
	f := newPermFixture(t)
	if _, err := f.db.AddOrgAdmin(t.Context(), iam.Actor{Kind: "system"}, f.orgA.ID, f.outsider.user.Email); err != nil {
		t.Fatal(err)
	}
	_, secret, err := f.db.CreateKey(t.Context(), iam.Actor{Kind: "system"}, iam.KeyInput{UserID: f.admin.user.ID, OwnerType: iam.OwnerPersonal})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		who  actor
	}{{"member", f.member}, {"team_admin", f.teamAdmin}, {"org_admin", f.outsider}, {"admin_key", actor{key: secret}}, {"master", actor{key: f.masterKey}}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, method := range []string{http.MethodGet, http.MethodPost} {
				path := "/budget/reservations"
				if method == http.MethodPost {
					path += "/missing/reconcile"
				}
				status, body := f.call(tc.who, method, path, []byte(`{}`))
				if status != 403 {
					t.Fatalf("%s %s越权或未明确拒绝: %d %s", tc.name, method, status, body)
				}
			}
		})
	}
	if status, body := f.call(f.admin, http.MethodGet, "/budget/reservations", nil); status != 200 {
		t.Fatalf("管理会话不可读取: %d %s", status, body)
	}
}

// TestBudgetReconciliationJSONContract 验证严格JSON请求拒绝未知字段、遗漏金额、多个对象、错误金额类型及过大正文。
// 前置真实管理session及空数据库；合法但不存在的尝试返回404，任何输入不创建核对结果，fixture结束清理。
func TestBudgetReconciliationJSONContract(t *testing.T) {
	f := newPermFixture(t)
	valid := `{"outcome":"no_charge","verified_cost_usd":0,"evidence":"Reviewed invoice","expected_fingerprint":"` + strings.Repeat("a", 64) + `"}`
	for name, body := range map[string]string{"trailing": valid + ` {}`, "unknown": strings.TrimSuffix(valid, "}") + `,"force":true}`, "missing_amount": `{"outcome":"no_charge"}`, "string_amount": strings.Replace(valid, `"verified_cost_usd":0`, `"verified_cost_usd":"0"`, 1), "too_large": strings.Repeat(" ", 16385) + valid, "invalid": `{"outcome":`} {
		t.Run(name, func(t *testing.T) {
			status, raw := f.call(f.admin, http.MethodPost, "/budget/reservations/missing/reconcile", []byte(body))
			if status != 400 || !strings.Contains(string(raw), "budget_reconciliation_invalid") {
				t.Fatalf("无效JSON未明确拒绝: %d %s", status, raw)
			}
		})
	}
	if status, body := f.call(f.admin, http.MethodPost, "/budget/reservations/missing/reconcile", []byte(valid)); status != 404 {
		t.Fatalf("不存在的尝试状态错误: %d %s", status, body)
	}
	n, err := f.db.Engine.Table("budget_reconciliations").Count()
	if err != nil || n != 0 {
		t.Fatalf("非法请求修改了账务: %d %v", n, err)
	}
}
