package iam

import (
	"context"
	"encoding/json"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/sunqirui1987/xhub/cmd/regression/testsupport"
)

// TestPendingUsageFreezesDefaultTimestamp 验证缺省时间在首次保存时固定，恢复不会把旧账单移到新周期。
// 前置私有数据库和可恢复约束失败，读取持久快照后恢复；原调用值保持不变，schema 自动回收。
func TestPendingUsageFreezesDefaultTimestamp(t *testing.T) {
	db := testDB(t)
	if _, err := db.Engine.Exec("ALTER TABLE usage_events ADD CONSTRAINT pending_time_reject CHECK(request_id <> 'pending-time')"); err != nil {
		t.Fatal(err)
	}
	record := usageRecord("pending-time", "", 0)
	record.TS = time.Time{}
	before := time.Now().Add(-time.Second)
	if durable, err := db.RecordUsageDurably(t.Context(), record); !durable || err == nil {
		t.Fatalf("未保留时间快照: durable=%t err=%v", durable, err)
	}
	var saved pendingUsage
	if found, err := db.Engine.Table("usage_pending").Where("request_id = ?", record.RequestID).Get(&saved); err != nil || !found {
		t.Fatalf("读取待结算快照失败: found=%t err=%v", found, err)
	}
	var snapshot UsageRecord
	if err := json.Unmarshal([]byte(saved.Payload), &snapshot); err != nil || snapshot.TS.Before(before) || snapshot.TS.After(time.Now()) || !record.TS.IsZero() {
		t.Fatalf("默认时间未冻结或修改调用方: %+v err=%v", snapshot, err)
	}
	if _, err := db.Engine.Exec("ALTER TABLE usage_events DROP CONSTRAINT pending_time_reject"); err != nil {
		t.Fatal(err)
	}
	if err := db.FlushPendingUsage(t.Context(), 1); err != nil {
		t.Fatal(err)
	}
	event, err := db.GetUsageEvent(t.Context(), UsageQuery{}, record.RequestID)
	// 现有 xorm 账单写入按整秒保存 ts；按该持久化精度比较，队列本身仍保留完整时间。
	if err != nil || !event.TS.Equal(snapshot.TS.Truncate(time.Second)) {
		t.Fatalf("恢复改变调用时间: %+v snapshot=%v err=%v", event, snapshot.TS, err)
	}
}

// pendingCount 查询私有测试库待结算数；参数为上下文/库，返回数量，读取失败终止测试，数据由 testDB 清理。
func pendingCount(t *testing.T, db *DB) int64 {
	t.Helper()
	n, err := db.Engine.Table("usage_pending").Count()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return n
}

// TestPendingUsageRecoveryAcrossInstances 验证账单事务失败保留完整快照，新数据库实例恢复且并发重放不重复收费。
// 前置私有 PostgreSQL schema，故障只施加于 request_logs；核对正文/归属/价格和金额，关闭连接并自动删除 schema。
func TestPendingUsageRecoveryAcrossInstances(t *testing.T) {
	dsn := testsupport.Postgres(t, "iam")
	db, err := Open(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	u, err := db.CreateUser(t.Context(), Actor{Kind: "system"}, UserInput{Email: "pending@example.com", Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	record := usageRecord("pending-full-snapshot", u.ID, 0.125)
	record.RequestBody, record.ResponseBody, record.ProxyRequest = `{"prompt":"Hello"}`, `{"text":"Hello"}`, `{"Authorization":"***"}`
	record.SessionID, record.SessionSource, record.PriceSnapshot = "pending-session", "explicit", `{"total_cost":0.125}`
	record.EndedAt, record.ReasoningEffort = record.TS.Add(time.Second), "high"
	if _, err = db.Engine.Exec("ALTER TABLE request_logs ADD CONSTRAINT reject_pending CHECK (request_id <> 'pending-full-snapshot')"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		durable, err := db.RecordUsageDurably(t.Context(), record)
		if !durable || err == nil || pendingCount(t, db) != 1 || countEvents(t, db, record.RequestID) != 0 || userSpend(t, db, u.ID) != 0 {
			t.Fatalf("事务失败未保存唯一快照/发生部分扣款: durable=%t err=%v", durable, err)
		}
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	if pendingCount(t, reopened) != 1 {
		t.Fatal("新实例丢失待结算快照")
	}
	if _, err = reopened.Engine.Exec("ALTER TABLE request_logs DROP CONSTRAINT reject_pending"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := reopened.FlushPendingUsage(context.Background(), 10); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if pendingCount(t, reopened) != 0 || countEvents(t, reopened, record.RequestID) != 1 || userSpend(t, reopened, u.ID) != 0.125 || dailyRequests(t, reopened, u.ID) != 1 {
		t.Fatal("恢复后待结算未清理或发生重复计费/汇总")
	}
	log, err := reopened.GetRequestLog(t.Context(), record.RequestID)
	if err != nil || log.RequestBody != record.RequestBody || log.ResponseBody != record.ResponseBody || log.ProxyRequest != record.ProxyRequest {
		t.Fatalf("恢复丢失完整正文: %+v %v", log, err)
	}
	event, err := reopened.GetUsageEvent(t.Context(), UsageQuery{}, record.RequestID)
	if err != nil || event.SessionID != record.SessionID || event.SessionSource != record.SessionSource || event.ReasoningEffort != record.ReasoningEffort || event.PriceSnapshot != record.PriceSnapshot {
		t.Fatalf("恢复丢失调用元数据: %+v %v", event, err)
	}
}

// TestPendingUsageValidationAndUnwritableStore 验证非法输入和队列表本身不可写不会被宣称已持久化。
// 前置私有库，覆盖空ID/负值/NaN/取消上下文/约束拒绝；不留下账单或队列，测试结束删除 schema。
func TestPendingUsageValidationAndUnwritableStore(t *testing.T) {
	db := testDB(t)
	negative := -1
	for _, record := range []UsageRecord{{}, {RequestID: "negative", Cost: -1}, {RequestID: "nan", Cost: math.NaN()}, {RequestID: "tokens", PromptTokens: -1}, {RequestID: "cached", CachedTokens: &negative}} {
		if durable, err := db.RecordUsageDurably(t.Context(), record); durable || err == nil {
			t.Fatalf("非法快照被入队: durable=%t err=%v", durable, err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if durable, err := db.RecordUsageDurably(ctx, usageRecord("canceled", "", 0)); durable || err == nil {
		t.Fatal("取消的保存误报持久化")
	}
	if _, err := db.Engine.Exec("ALTER TABLE usage_pending ADD CONSTRAINT reject_all CHECK (false)"); err != nil {
		t.Fatal(err)
	}
	if durable, err := db.RecordUsageDurably(t.Context(), usageRecord("unwritable", "", 0)); durable || err == nil {
		t.Fatal("队列写入失败误报持久化")
	}
	if pendingCount(t, db) != 0 || countEvents(t, db, "unwritable") != 0 {
		t.Fatal("队列保存失败仍发布了账单")
	}
	if db.FlushPendingUsage(t.Context(), 0) == nil {
		t.Fatal("非法批次大小未拒绝")
	}
}

// TestPendingUsagePoisonAndAckRecovery 验证损坏快照留存且不阻塞同批有效记录，提交后确认失败可幂等恢复。
// 前置私有库，用合法 JSON 的错误身份制造损坏条目，用删除触发器模拟确认失败；清理触发器和 schema。
func TestPendingUsagePoisonAndAckRecovery(t *testing.T) {
	db := testDB(t)
	if _, err := db.Engine.Exec(`INSERT INTO usage_pending(request_id,payload) VALUES ('broken','{"RequestID":"other"}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Engine.Exec(`CREATE FUNCTION reject_pending_delete() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'test ack failure'; END $$ LANGUAGE plpgsql; CREATE TRIGGER pending_delete BEFORE DELETE ON usage_pending FOR EACH ROW EXECUTE FUNCTION reject_pending_delete()`); err != nil {
		t.Fatal(err)
	}
	record := usageRecord("committed-before-ack", "", 0.2)
	if durable, err := db.RecordUsageDurably(t.Context(), record); !durable || err == nil || countEvents(t, db, record.RequestID) != 1 || pendingCount(t, db) != 2 {
		t.Fatalf("确认失败没有保留已结算快照: durable=%t err=%v", durable, err)
	}
	if _, err := db.Engine.Exec("DROP TRIGGER pending_delete ON usage_pending; DROP FUNCTION reject_pending_delete()"); err != nil {
		t.Fatal(err)
	}
	if err := db.FlushPendingUsage(t.Context(), 10); err == nil {
		t.Fatal("损坏快照没有明确错误")
	}
	if pendingCount(t, db) != 1 || countEvents(t, db, record.RequestID) != 1 {
		t.Fatal("损坏条目被删除或正常条目未独立恢复")
	}
	if err := db.Migrate(t.Context()); err != nil || pendingCount(t, db) != 1 {
		t.Fatalf("重复迁移丢失待修复快照: %v", err)
	}
}
