package regression

import (
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/sunqirui1987/xhub/internal/iam"
)

// rawBudgetUpdate 发送未经重新序列化的原始JSON；参数为harness、路径、凭据和原文，返回实际HTTP结果。
// 用于验证截断、多JSON与溢出，标准JSON编码器不能构造这些正文；关闭响应体，不清理业务对象，调用方harness负责schema回收。
func rawBudgetUpdate(t *testing.T, h *harness, path, token, raw string) reply {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, h.server.URL+path, strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := h.server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return reply{status: response.StatusCode, body: body, headers: response.Header}
}

// TestBudgetUpdateJSONAtomicity 验证真实用户/Key更新拒绝非法JSON和错误额度类型，且名称、额度、消费、会话/到期与审计均不变。
// 前置私有PostgreSQL和两类独立账户/Key；正常零/null、数字及兼容数字字符串作控制，harness删除所有合成数据。
func TestBudgetUpdateJSONAtomicity(t *testing.T) {
	for _, kind := range []string{"user", "key"} {
		t.Run(kind, func(t *testing.T) {
			h := newHarness(t)
			admin := h.adminSession()
			owner := h.provision(t, admin, "strict-json-"+kind)
			path, idField, id, aliasField := "/user/update", "user_id", owner.userID, "user_alias"
			if kind == "key" {
				key, err := h.db.KeyByHash(t.Context(), iam.HashKey(owner.key))
				if err != nil || key == nil {
					t.Fatalf("fixture key: %v", err)
				}
				path, idField, id, aliasField = "/key/update", "key", key.ID, "key_alias"
			}
			h.ok(http.MethodPost, path, admin, map[string]any{idField: id, "max_budget": 10, aliasField: "Original 用户"})
			userBefore, err := h.db.GetUser(t.Context(), owner.userID)
			if err != nil {
				t.Fatal(err)
			}
			var keyBefore *iam.Key
			if kind == "key" {
				keyBefore, err = h.db.GetKey(t.Context(), id)
				if err != nil {
					t.Fatal(err)
				}
			}
			initialAudit, err := h.db.Engine.Where("action = ? AND object_id = ?", kind+".update", id).Count(new(iam.AuditEntry))
			if err != nil {
				t.Fatal(err)
			}
			prefix := fmt.Sprintf(`{%q:%q,%q:"must-not-save","max_budget":`, idField, id, aliasField)
			for _, tc := range []struct{ name, raw string }{
				{"positive overflow", prefix + `1e309}`}, {"negative overflow", prefix + `-1e309}`},
				{"truncated", prefix + `null`}, {"second JSON", prefix + `null} {}`}, {"trailing garbage", prefix + `null} broken`},
				{"nonobject array", `[]`}, {"nonobject null", `null`}, {"empty", ``},
				{"oversized", prefix + `null}` + strings.Repeat(" ", 1<<20)},
				{"invalid string", prefix + `"not-a-number"}`}, {"overflow string", prefix + `"1e309"}`},
				{"NaN string", prefix + `"NaN"}`}, {"Infinity string", prefix + `"Infinity"}`},
				{"object budget", prefix + `{}}`}, {"array budget", prefix + `[]}`}, {"boolean budget", prefix + `false}`},
			} {
				t.Run(tc.name, func(t *testing.T) {
					response := rawBudgetUpdate(t, h, path, admin, tc.raw)
					if response.status != 400 || response.json()["error"].(map[string]any)["type"] != "invalid_request" {
						t.Fatalf("坏JSON/额度类型未拒绝: %s", response.describe())
					}
					user, err := h.db.GetUser(t.Context(), owner.userID)
					if err != nil {
						t.Fatal(err)
					}
					if user.Name != userBefore.Name || user.Spend != userBefore.Spend || user.Role != userBefore.Role || user.SessionVersion != userBefore.SessionVersion {
						t.Fatalf("失败更新修改用户副字段: %+v", user)
					}
					if kind == "user" {
						if user.MaxBudget == nil || *user.MaxBudget != 10 {
							t.Fatal("失败请求取消或修改用户额度")
						}
					} else {
						key, err := h.db.GetKey(t.Context(), id)
						if err != nil {
							t.Fatal(err)
						}
						if key.MaxBudget == nil || *key.MaxBudget != 10 || key.Name != keyBefore.Name || key.Spend != keyBefore.Spend || !reflect.DeepEqual(key.ExpiresAt, keyBefore.ExpiresAt) {
							t.Fatalf("失败请求修改Key字段: %+v", key)
						}
					}
					audit, err := h.db.Engine.Where("action = ? AND object_id = ?", kind+".update", id).Count(new(iam.AuditEntry))
					if err != nil || audit != initialAudit {
						t.Fatalf("拒绝留下审计: before=%d after=%d err=%v", initialAudit, audit, err)
					}
				})
			}
			for _, control := range []struct {
				name  string
				value any
				clear bool
				want  float64
			}{{"zero", 0, false, 0}, {"positive", 10, false, 10}, {"numeric string", "5", false, 5}, {"explicit null", nil, true, 0}, {"legacy empty string", "", true, 0}} {
				t.Run("valid "+control.name, func(t *testing.T) {
					h.ok(http.MethodPost, path, admin, map[string]any{idField: id, "max_budget": control.value})
					var budget *float64
					if kind == "user" {
						u, err := h.db.GetUser(t.Context(), id)
						if err != nil {
							t.Fatal(err)
						}
						budget = u.MaxBudget
					} else {
						k, err := h.db.GetKey(t.Context(), id)
						if err != nil {
							t.Fatal(err)
						}
						budget = k.MaxBudget
					}
					if (budget == nil) != control.clear || budget != nil && *budget != control.want {
						t.Fatalf("合法预算语义错误: %v", budget)
					}
				})
			}
			// 最后删除自建账号并检查其Key失效；测试schema回收剩余组织、团队、审计。
			h.ok(http.MethodPost, "/user/delete", admin, map[string]any{"user_id": owner.userID})
			if response := h.do(http.MethodGet, "/v1/models", owner.key, nil); response.status != 401 {
				t.Fatalf("清理后Key仍有效: %s", response.describe())
			}
		})
	}
}
