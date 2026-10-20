package iam

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"sync"
	"testing"
	"time"
)

// reconciliationFixture 创建私有PG中的管理员、五层身份和同call两个未知尝试；原账单已记录0.2美元。
// 参数t提供失败上下文，返回可核对身份/账单/两个尝试；不连接供应商，testDB在结束后删除整个schema。
func reconciliationFixture(t *testing.T) (*DB, Actor, UsageRecord, []string) {
	t.Helper()
	db := testDB(t)
	ctx := t.Context()
	if _, err := db.EnsureAdmin(ctx, "reconcile-admin@example.com", "Admin", "password123"); err != nil {
		t.Fatal(err)
	}
	admin, err := db.UserByEmail(ctx, "reconcile-admin@example.com")
	if err != nil {
		t.Fatal(err)
	}
	by := Actor{ID: admin.ID, Kind: "session"}
	org, err := db.CreateOrgWithConfig(ctx, Actor{}, "reconcile-org", quotaLimit(100), OrgConfig{BudgetDuration: "24h"})
	if err != nil {
		t.Fatal(err)
	}
	team, err := db.CreateTeam(ctx, Actor{}, TeamInput{OrganizationID: org.ID, Name: "reconcile-team"})
	if err != nil {
		t.Fatal(err)
	}
	project, err := db.CreateProject(ctx, Actor{}, ProjectInput{TeamID: team.ID, Name: "reconcile-project"})
	if err != nil {
		t.Fatal(err)
	}
	user, err := db.CreateUser(ctx, Actor{}, UserInput{Email: "holder@example.com", Password: "password123", TeamID: team.ID})
	if err != nil {
		t.Fatal(err)
	}
	key, _, err := db.CreateKey(ctx, Actor{}, KeyInput{OwnerType: OwnerPersonal, UserID: user.ID, TeamID: team.ID, ProjectID: project.ID})
	if err != nil {
		t.Fatal(err)
	}
	r := UsageRecord{RequestID: "reconcile-call", KeyID: key.ID, UserID: user.ID, TeamID: team.ID, OrganizationID: org.ID, ProjectID: project.ID, OwnerType: OwnerPersonal, Model: "synthetic", Supplier: "local", UpstreamModel: "actual", TS: time.Now().UTC().Truncate(time.Microsecond), Status: "error", HTTPStatus: 502, Cost: 0.2, PromptTokens: 3, PriceSnapshot: `{"original":true}`}
	ids := []string{"reconcile-attempt-a", "reconcile-attempt-b"}
	for _, id := range ids {
		if _, err = db.ReserveBudget(ctx, id, "key", key.ID, 1, BudgetReservationOptions{CallID: r.RequestID, StartedAt: r.TS, Snapshot: `{"fixed":true,"model":"actual","supplier":"local"}`}); err != nil {
			t.Fatal(err)
		}
		if err = db.SetBudgetReservationState(ctx, id, "uncertain", "synthetic incomplete usage"); err != nil {
			t.Fatal(err)
		}
	}
	if err = db.RecordUsage(ctx, []UsageRecord{r}); err != nil {
		t.Fatal(err)
	}
	return db, by, r, ids
}

// reconciliationInput 读取当前尝试的指纹并构造核实请求；参数cost为该尝试增量，不包含call已消费金额。
// 返回合法输入，缺失/不可操作记录使测试失败；仅读私有schema，不创建额外资源。
func reconciliationInput(t *testing.T, db *DB, id string, cost float64) BudgetReconciliationInput {
	t.Helper()
	rows, _, err := db.ListBudgetReservations(t.Context(), "", "", 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.ReservationID == id {
			outcome := "charged"
			if cost == 0 {
				outcome = "no_charge"
			}
			return BudgetReconciliationInput{Outcome: outcome, VerifiedCost: cost, Evidence: "Synthetic provider invoice reviewed", ExpectedFingerprint: row.Fingerprint}
		}
	}
	t.Fatal("待核对尝试缺失")
	return BudgetReconciliationInput{}
}

// TestBudgetReconciliationCostOnlyAndLateFence 验证同call多尝试只增加实际费用，允许超预占，不增加请求或token。
// 前置完整五层fixture；覆盖重复操作、同金额晚到、冲突晚到保留PG pending、原证据与审计留存，schema自动清理。
func TestBudgetReconciliationCostOnlyAndLateFence(t *testing.T) {
	db, by, r, ids := reconciliationFixture(t)
	ctx := t.Context()
	input := reconciliationInput(t, db, ids[0], 2)
	resolved, already, err := db.ReconcileBudgetReservation(ctx, by, ids[0], input)
	if err != nil || already || resolved.PreviousCost != 0.2 || resolved.UpdatedCost != 2.2 {
		t.Fatalf("已结算费用被覆盖或不能超预占核实: %+v %v %v", resolved, already, err)
	}
	if _, again, err := db.ReconcileBudgetReservation(ctx, by, ids[0], input); err != nil || !again {
		t.Fatalf("相同结论不幂等: %v %v", again, err)
	}
	changed := input
	changed.Evidence = "Different invoice"
	if _, _, err = db.ReconcileBudgetReservation(ctx, by, ids[0], changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("同尝试不同依据未拒绝: %v", err)
	}
	if _, _, err = db.ReconcileBudgetReservation(ctx, by, ids[1], reconciliationInput(t, db, ids[1], 0.3)); err != nil {
		t.Fatal(err)
	}
	var event UsageEvent
	if found, err := db.Engine.Where("request_id = ?", r.RequestID).Get(&event); err != nil || !found || event.Cost != 2.5 || event.PromptTokens != 3 || event.PriceSnapshot != r.PriceSnapshot {
		t.Fatalf("原账单或实际费用损坏: %+v %v", event, err)
	}
	var daily UsageDaily
	if found, err := db.Engine.Where("key_id = ?", r.KeyID).Get(&daily); err != nil || !found || daily.Requests != 1 || daily.PromptTokens != 3 || daily.Cost != 2.5 {
		t.Fatalf("日汇总虚增请求/Token: %+v %v", daily, err)
	}
	var monitor struct {
		Requests     int64
		PromptTokens int64 `xorm:"prompt_tokens"`
		Cost         float64
	}
	if _, err = db.Engine.Table("model_monitor_buckets").Where("key_id = ?", r.KeyID).Get(&monitor); err != nil || monitor.Requests != 1 || monitor.PromptTokens != 3 || monitor.Cost != 2.5 {
		t.Fatalf("监控虚增: %+v %v", monitor, err)
	}
	for table, id := range map[string]string{"api_keys": r.KeyID, "users": r.UserID, "teams": r.TeamID, "projects": r.ProjectID, "organizations": r.OrganizationID} {
		var value struct{ Spend float64 }
		if _, err = db.Engine.Table(table).Where("id = ?", id).Get(&value); err != nil || value.Spend != 2.5 {
			t.Fatalf("%s消费=%v err=%v", table, value.Spend, err)
		}
	}
	if err = db.RecordUsage(ctx, []UsageRecord{r}); err != nil {
		t.Fatalf("原失败快照重复确认失败: %v", err)
	}
	late := r
	late.BudgetReservationID = ids[0]
	late.Cost = 2
	if err = db.RecordUsage(ctx, []UsageRecord{late}); err != nil {
		t.Fatalf("一致晚到重复扣费或拒绝: %v", err)
	}
	late.Cost = 2.1
	if durable, err := db.RecordUsageDurably(ctx, late); !durable || !errors.Is(err, ErrConflict) {
		t.Fatalf("冲突晚到证据未持久保留: durable=%v err=%v", durable, err)
	}
	pending, _ := db.Engine.Table("usage_pending").Count()
	if pending != 1 {
		t.Fatalf("冲突队列被错误确认: %d", pending)
	}
	n, _ := db.Engine.Where("action = ?", "budget.reconcile").Count(new(AuditEntry))
	if n != 2 {
		t.Fatalf("重复核销虚增审计: %d", n)
	}
	s := db.session(ctx)
	defer s.Close()
	saved, found, err := readBudgetResolution(s, ids[0])
	if err != nil || !found || saved.Event.Cost != 0.2 || saved.Hold.Snapshot != `{"fixed":true,"model":"actual","supplier":"local"}` {
		t.Fatalf("原证据被覆盖: %+v %v", saved, err)
	}
	if err = db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	n, _ = db.Engine.Table("budget_reconciliations").Count()
	if n != 2 {
		t.Fatal("重复迁移删除核对证据")
	}
}

// TestBudgetReconciliationSafetyAndRollback 验证缺账单、在途、待结算、坏指纹与审计失败均不释放未知占用。
// 前置私有PG五层fixture；注入仅本schema审计约束，验证完整回滚，结束自动删除schema。
func TestBudgetReconciliationSafetyAndRollback(t *testing.T) {
	for _, name := range []string{"settling", "missing_event", "active_attempt", "pending_usage", "bad_fingerprint", "audit_failure", "non_admin"} {
		t.Run(name, func(t *testing.T) {
			db, by, r, ids := reconciliationFixture(t)
			ctx := t.Context()
			in := reconciliationInput(t, db, ids[0], 0)
			want := ErrConflict
			switch name {
			case "settling":
				if err := db.SetBudgetReservationState(ctx, ids[0], "settling"); err != nil {
					t.Fatal(err)
				}
			case "missing_event":
				if _, err := db.Engine.Where("request_id = ?", r.RequestID).Delete(new(UsageEvent)); err != nil {
					t.Fatal(err)
				}
			case "active_attempt":
				if err := db.SetBudgetReservationState(ctx, ids[1], "submitted"); err != nil {
					t.Fatal(err)
				}
			case "pending_usage":
				raw, _ := json.Marshal(r)
				if _, err := db.Engine.Exec("INSERT INTO usage_pending(request_id,payload) VALUES (?,?::jsonb)", r.RequestID, string(raw)); err != nil {
					t.Fatal(err)
				}
			case "bad_fingerprint":
				in.ExpectedFingerprint = strings.Repeat("a", 64)
			case "audit_failure":
				if _, err := db.Engine.Exec("ALTER TABLE audit_logs ADD CONSTRAINT reject_reconcile CHECK (action <> 'budget.reconcile')"); err != nil {
					t.Fatal(err)
				}
				want = nil
			case "non_admin":
				by.ID = r.UserID
				want = ErrInvalid
			}
			_, _, err := db.ReconcileBudgetReservation(ctx, by, ids[0], in)
			if err == nil || (want != nil && !errors.Is(err, want)) {
				t.Fatalf("%s未安全拒绝: %v", name, err)
			}
			holds, _ := db.PendingKeyBudget(ctx, r.KeyID)
			if len(holds) != 2 {
				t.Fatalf("拒绝后释放预占: %+v", holds)
			}
			n, _ := db.Engine.Table("budget_reconciliations").Count()
			if n != 0 {
				t.Fatal("失败事务留下人工终态")
			}
		})
	}
}

// TestBudgetReconciliationConcurrentAndPeriod 验证并发相同操作仅一次核销，旧周期核实不污染当前组织余额。
// 前置同call两尝试与手动私有周期切换；两个尝试独立追加到其他四层，撤销Key不妨碍历史核对，schema自动清理。
func TestBudgetReconciliationConcurrentAndPeriod(t *testing.T) {
	db, by, r, ids := reconciliationFixture(t)
	ctx := t.Context()
	boundary := r.TS.Add(time.Second)
	if _, err := db.Engine.Exec("UPDATE organizations SET budget_period_start = ?, spend=0 WHERE id = ?", boundary, r.OrganizationID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Engine.Exec("UPDATE api_keys SET status = 'revoked' WHERE id = ?", r.KeyID); err != nil {
		t.Fatal(err)
	}
	input := reconciliationInput(t, db, ids[0], 0.4)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	fresh := make(chan bool, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, again, err := db.ReconcileBudgetReservation(ctx, by, ids[0], input)
			errs <- err
			fresh <- !again
		}()
	}
	wg.Wait()
	close(errs)
	close(fresh)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	count := 0
	for value := range fresh {
		if value {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("并发产生%d份核销", count)
	}
	org, err := db.GetOrg(ctx, r.OrganizationID)
	if err != nil || org.Spend != 0 {
		t.Fatalf("旧周期费用污染新周期: %+v %v", org, err)
	}
	// 同call第二笔仍按原周期核对；第一笔已核销不能覆盖或释放第二笔的独立预占。
	if _, _, err := db.ReconcileBudgetReservation(ctx, by, ids[1], reconciliationInput(t, db, ids[1], 0.3)); err != nil {
		t.Fatal(err)
	}
	for table, id := range map[string]string{"api_keys": r.KeyID, "users": r.UserID, "teams": r.TeamID, "projects": r.ProjectID, "organizations": r.OrganizationID} {
		want := 0.9
		if table == "organizations" {
			want = 0
		}
		var value struct{ Spend float64 }
		if _, err = db.Engine.Table(table).Where("id = ?", id).Get(&value); err != nil || math.Abs(value.Spend-want) > 1e-12 {
			t.Fatalf("跨周期多尝试%s消费=%v，预期%v err=%v", table, value.Spend, want, err)
		}
	}
	var event UsageEvent
	if _, err = db.Engine.Where("request_id = ?", r.RequestID).Get(&event); err != nil || math.Abs(event.Cost-0.9) > 1e-12 || event.PromptTokens != r.PromptTokens {
		t.Fatalf("跨周期历史账单未保留两次费用与原Token: %+v %v", event, err)
	}
}

// TestBudgetReconciliationInputBoundaries 验证纯函数拒绝非法金额、缺依据、未知结论和非规范指纹。
// 前置无外部服务；覆盖正常、零收费、负数/NaN/Inf/错误类型语义，纯内存无需清理。
func TestBudgetReconciliationInputBoundaries(t *testing.T) {
	valid := BudgetReconciliationInput{Outcome: "charged", VerifiedCost: 0.1, Evidence: "  invoice  ", ExpectedFingerprint: strings.Repeat("a", 64)}
	if got, err := validateBudgetReconciliation(valid); err != nil || got.Evidence != "invoice" {
		t.Fatalf("正常依据校验失败: %+v %v", got, err)
	}
	for _, name := range []string{"negative", "nan", "inf", "empty", "long", "fingerprint", "uppercase", "outcome", "zero_charge", "nonzero_free"} {
		t.Run(name, func(t *testing.T) {
			in := valid
			switch name {
			case "negative":
				in.VerifiedCost = -1
			case "nan":
				in.VerifiedCost = math.NaN()
			case "inf":
				in.VerifiedCost = math.Inf(1)
			case "empty":
				in.Evidence = " "
			case "long":
				in.Evidence = strings.Repeat("a", 2001)
			case "fingerprint":
				in.ExpectedFingerprint = "short"
			case "uppercase":
				in.ExpectedFingerprint = strings.Repeat("A", 64)
			case "outcome":
				in.Outcome = "refund"
			case "zero_charge":
				in.VerifiedCost = 0
			case "nonzero_free":
				in.Outcome = "no_charge"
			}
			if _, err := validateBudgetReconciliation(in); !errors.Is(err, ErrInvalid) {
				t.Fatal("非法核对输入被接受")
			}
		})
	}
}

// TestBudgetReconciliationLateBeforeManual 验证已有失败事件时晚到结算不能先释放hold再被call去重吞掉费用。
// 前置五层PG fixture，模拟自动结算先到；返回冲突且原费用和两条占用不变，无外部上游，schema自动清理。
func TestBudgetReconciliationLateBeforeManual(t *testing.T) {
	db, _, r, ids := reconciliationFixture(t)
	late := r
	late.BudgetReservationID = ids[0]
	late.Cost = 0.3
	if err := db.RecordUsage(t.Context(), []UsageRecord{late}); !errors.Is(err, ErrConflict) {
		t.Fatalf("重复call的新尝试结算未保留冲突: %v", err)
	}
	holds, err := db.PendingKeyBudget(t.Context(), r.KeyID)
	if err != nil || len(holds) != 2 {
		t.Fatalf("冲突晚到释放未知占用: %+v %v", holds, err)
	}
	key, err := db.GetKey(t.Context(), r.KeyID)
	if err != nil || key.Spend != 0.2 {
		t.Fatalf("冲突晚到改变原费用: %+v %v", key, err)
	}
}

// TestBudgetReconciliationDeletedIdentityAndZero 验证身份删除后仍按冻结归属核销历史，零费用核实不会新增消费。
// 前置私有PG fixture，直接模拟历史身份被删除；原event和核对证据留存，schema自动清理。
func TestBudgetReconciliationDeletedIdentityAndZero(t *testing.T) {
	db, by, r, ids := reconciliationFixture(t)
	ctx := t.Context()
	input := reconciliationInput(t, db, ids[0], 0)
	if _, err := db.Engine.Exec("DELETE FROM api_keys WHERE id = ?", r.KeyID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Engine.Exec("DELETE FROM team_members WHERE user_id = ?", r.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Engine.Exec("DELETE FROM users WHERE id = ?", r.UserID); err != nil {
		t.Fatal(err)
	}
	result, _, err := db.ReconcileBudgetReservation(ctx, by, ids[0], input)
	if err != nil || result.CostDelta != 0 || result.PreviousCost != result.UpdatedCost {
		t.Fatalf("历史身份删除影响零费用核实: %+v %v", result, err)
	}
	var event UsageEvent
	if found, err := db.Engine.Where("request_id = ?", r.RequestID).Get(&event); err != nil || !found || event.Cost != 0.2 || event.UserID != r.UserID {
		t.Fatalf("历史归属或费用被改写: %+v %v", event, err)
	}
}

// TestBudgetReconciliationUnknownAttemptSource 验证旧预占没有供应商时标记未知，不将费用误记到最后成功连接。
// 前置PG fixture，删除仅测试hold快照的supplier；检查列表与成本桶保持未知，其他统计不变，schema自动清理。
func TestBudgetReconciliationUnknownAttemptSource(t *testing.T) {
	db, by, r, ids := reconciliationFixture(t)
	if _, err := db.Engine.Exec("UPDATE budget_reservations SET snapshot = ? WHERE request_id = ?", `{"model":"earlier-model"}`, ids[0]); err != nil {
		t.Fatal(err)
	}
	rows, _, err := db.ListBudgetReservations(t.Context(), r.KeyID, "uncertain", 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.ReservationID == ids[0] && (row.Supplier != "" || row.UpstreamModel != "earlier-model") {
			t.Fatalf("旧attempt冒充最后供应商: %+v", row)
		}
	}
	if _, _, err = db.ReconcileBudgetReservation(t.Context(), by, ids[0], reconciliationInput(t, db, ids[0], 0.3)); err != nil {
		t.Fatal(err)
	}
	var bucket struct {
		Cost     float64
		Requests int64
	}
	if found, err := db.Engine.Table("model_monitor_buckets").Where("key_id=? AND supplier='' AND model='earlier-model'", r.KeyID).Get(&bucket); err != nil || !found || bucket.Cost != 0.3 || bucket.Requests != 0 {
		t.Fatalf("未知供应商增量桶错误: %+v %v", bucket, err)
	}
}
