package gateway

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sunqirui1987/xhub/internal/auth"
	"github.com/sunqirui1987/xhub/internal/config"
	"github.com/sunqirui1987/xhub/internal/iam"
)

// TestBudgetAttemptLifecycle 单独验证网关预占模块的正常结算、未发送退款、取消与不可信用量保留。
// 私有数据库只提供持久依赖，不启动HTTP上游；各子例独立身份，清理由testGatewayStores完成。
func TestBudgetAttemptLifecycle(t *testing.T) {
	for _, mode := range []string{"unsent", "cancelled", "valid", "missing", "overflow", "unsupported"} {
		t.Run(mode, func(t *testing.T) {
			cfg, st, db := testGatewayStores(t)
			s := New(cfg, st, db)
			limit := 1.0
			user, err := db.CreateUser(t.Context(), iam.Actor{}, iam.UserInput{Email: "budget-" + mode + "@example.com", Password: "password123", MaxBudget: &limit})
			if err != nil {
				t.Fatal(err)
			}
			key, _, err := db.CreateKey(t.Context(), iam.Actor{}, iam.KeyInput{UserID: user.ID, OwnerType: iam.OwnerPersonal})
			if err != nil {
				t.Fatal(err)
			}
			p := &auth.Principal{UserID: user.ID, KeyID: key.ID, Key: key}
			dep := config.ModelEntry{ModelName: "budget-unit", LiteLLMParams: map[string]any{"input_cost_per_token": 0.000002, "output_cost_per_token": 0.000008}, ModelInfo: map[string]any{"max_input_tokens": 1024, "max_output_tokens": 8192, "supported_parameters": []string{"max_tokens"}, "pricing_source": "manual"}}
			body := []byte(`{"messages":[{"role":"user","content":"Test budget."}]}`)
			if mode == "unsupported" {
				delete(dep.ModelInfo, "max_input_tokens")
			}
			r := httptest.NewRequest("POST", "/v1/chat/completions", nil)
			w := httptest.NewRecorder()
			encoded, ok := s.PrepareBudgetAttempt(w, r, p, "call", dep, "openai-chat", body, time.Now())
			if mode == "unsupported" {
				if ok || w.Code != 400 {
					t.Fatalf("未知上界未拒绝: %t %d", ok, w.Code)
				}
				return
			}
			if !ok {
				t.Fatalf("正常预占失败: %d %s", w.Code, w.Body.String())
			}
			var out map[string]any
			_ = json.Unmarshal(encoded, &out)
			if out["max_tokens"] != float64(4096) {
				t.Fatalf("实际正文未带cap: %s", encoded)
			}
			if mode == "cancelled" {
				ctx, cancel := context.WithCancel(r.Context())
				cancel()
				r = r.WithContext(ctx)
				if s.MarkBudgetAttemptSubmitted(w, r, "call") {
					t.Fatal("取消请求被提交")
				}
			}
			if mode != "unsent" && mode != "cancelled" {
				if !s.MarkBudgetAttemptSubmitted(w, r, "call") {
					t.Fatal("正常提交失败")
				}
				usage := map[string]any{"prompt_tokens": 11, "completion_tokens": 5}
				if mode == "missing" {
					delete(usage, "completion_tokens")
				}
				if mode == "overflow" {
					usage["prompt_tokens"] = 2048
				}
				charge, active, valid, id, metadata := s.budgetCharge("call", usage, 200, false)
				if !active {
					t.Fatal("预占丢失")
				}
				if mode == "valid" {
					if !valid || id == "" || charge.Total <= 0 || metadata["state"] != "settling" {
						t.Fatalf("正常结算返回错误: %+v %v %v %s", charge, active, valid, id)
					}
					if err = db.RecordUsage(t.Context(), []iam.UsageRecord{{RequestID: "call", BudgetReservationID: id, KeyID: key.ID, UserID: user.ID, Cost: charge.Total, TS: time.Now()}}); err != nil {
						t.Fatal(err)
					}
				} else if valid || id != "" || metadata["reason"] == "" {
					t.Fatalf("未知用量误退款: %v %s %+v", valid, id, metadata)
				}
			}
			s.finishBudgetCall("call")
			s.finishBudgetCall("call")
			holds, err := db.PendingKeyBudget(t.Context(), key.ID)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "missing" || mode == "overflow" {
				if len(holds) != 1 || holds[0].State != "uncertain" || holds[0].Reason == "" {
					t.Fatalf("未知占用未保留: %+v", holds)
				}
			} else if len(holds) != 0 {
				t.Fatalf("已结算或未发送占用未释放: %+v", holds)
			}
		})
	}
}

// TestBudgetPriceSnapshot 验证空预算兼容、合法JSON合并与损坏/空快照；纯转换不写存储无需清理。
func TestBudgetPriceSnapshot(t *testing.T) {
	if got := budgetPriceSnapshot("original", nil); got != "original" {
		t.Fatal(got)
	}
	for _, raw := range []string{`{"price":1}`, `invalid`, `null`} {
		got := budgetPriceSnapshot(raw, map[string]any{"state": "uncertain"})
		var fields map[string]any
		if json.Unmarshal([]byte(got), &fields) != nil || fields["budget"] == nil {
			t.Fatalf("预算状态丢失: %s", got)
		}
	}
}
