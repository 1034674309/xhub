package live

import (
	"encoding/json"
	"testing"
)

// TestKeyPendingSpend 验证稳定 KeyID 索引跨旧新哈希累计、去重、确认及旧 writer 回退。
// 前置专用 Redis；每步核对真实队列与金额，testRedis 自动关闭连接并回收进程，无外部凭据。
func TestKeyPendingSpend(t *testing.T) {
	c := testRedis(t)
	check := func(id string, want float64) {
		t.Helper()
		got, err := c.HotKeySpend(id)
		if err != nil || got != want {
			t.Fatalf("Key %s 热费用=%v err=%v，期望 %v", id, got, err, want)
		}
	}
	check("key", 0)
	rows := []SpendLog{
		{RequestID: "before", KeyID: "key", APIKey: "old-hash", Spend: 2, SpendValid: true},
		{RequestID: "after", KeyID: "key", APIKey: "new-hash", Spend: 3, SpendValid: true},
		{RequestID: "other", KeyID: "other", APIKey: "other-hash", Spend: 7, SpendValid: true},
		{RequestID: "session", UserID: "user", Spend: 4, SpendValid: true},
	}
	for _, row := range rows {
		if err := c.EnqueueSpend(row); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.EnqueueSpend(rows[0]); err != nil {
		t.Fatal(err)
	}
	check("key", 5)
	check("other", 7)
	legacy := SpendLog{RequestID: "legacy", KeyID: "key", APIKey: "old-hash", Spend: 11, SpendValid: true}
	raw, _ := json.Marshal(legacy)
	if err := c.rdb.RPush(t.Context(), "xhub:spendlog", raw).Err(); err != nil {
		t.Fatal(err)
	}
	check("key", 16)
	check("other", 7)
	_, queued := c.PeekLogs(10)
	if err := c.AckFlushed(map[string]float64{"key/old-hash": 2, "key/new-hash": 3, "key/other-hash": 7, "user/user": 4}, len(queued), queued[0]); err != nil {
		t.Fatal(err)
	}
	check("key", 0)
	for _, key := range []string{"xhub:spend:key-indexed", "xhub:spend:key-pending:key", "xhub:spend:key-pending:other"} {
		if c.rdb.Exists(t.Context(), key).Val() != 0 {
			t.Fatalf("确认后遗留索引 %s", key)
		}
	}
	if err := c.EnqueueSpend(rows[0]); err != nil {
		t.Fatal(err)
	}
	check("key", 0)
	// 有费用的历史 Key 记录不能因缺失归属被当作免费；正常 Flush 可恢复此状态。
	legacy.RequestID, legacy.KeyID = "missing-key-id", ""
	if err := c.EnqueueSpend(legacy); err != nil {
		t.Fatal(err)
	}
	if _, err := c.HotKeySpend("key"); err == nil {
		t.Fatal("未知归属费用被静默忽略")
	}
}

// TestKeyPendingFailureAtomicity 验证损坏覆盖/主体索引和金额不会导致部分入队或确认。
// 前置专用 Redis；预检失败后原费用与队列不变，损坏读取显式错误；夹具清理全部资源。
func TestKeyPendingFailureAtomicity(t *testing.T) {
	for _, index := range []string{"xhub:spend:key-indexed", "xhub:spend:key-pending:key"} {
		t.Run(index, func(t *testing.T) {
			c := testRedis(t)
			ctx := t.Context()
			row := SpendLog{RequestID: "first", KeyID: "key", APIKey: "hash", Spend: 2, SpendValid: true}
			if err := c.EnqueueSpend(row); err != nil {
				t.Fatal(err)
			}
			_, raw := c.PeekLogs(10)
			if err := c.rdb.Set(ctx, index, "broken", 0).Err(); err != nil {
				t.Fatal(err)
			}
			row.RequestID = "second"
			if err := c.EnqueueSpend(row); err == nil {
				t.Fatal("损坏索引未拒绝入队")
			}
			if err := c.AckFlushed(map[string]float64{"key/hash": 2}, 1, raw[0]); err == nil {
				t.Fatal("损坏索引未拒绝确认")
			}
			if c.HotSpend("key/hash") != 2 || c.rdb.LLen(ctx, "xhub:spendlog").Val() != 1 {
				t.Fatal("预检失败仍修改金额或队列")
			}
			if _, err := c.HotKeySpend("key"); err == nil {
				t.Fatal("损坏索引读取未报错")
			}
		})
	}
	c := testRedis(t)
	row := SpendLog{RequestID: "cost", KeyID: "key", APIKey: "hash", Spend: 2, SpendValid: true}
	if err := c.EnqueueSpend(row); err != nil {
		t.Fatal(err)
	}
	if err := c.rdb.HSet(t.Context(), "xhub:spend:requests", "cost", "broken").Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.HotKeySpend("key"); err == nil {
		t.Fatal("损坏金额读取未报错")
	}
	if err := c.rdb.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.HotKeySpend("key"); err == nil {
		t.Fatal("Redis 失联不能显示零消费")
	}
	if got, err := (*Client)(nil).HotKeySpend("key"); got != 0 || err != nil {
		t.Fatal("无 Redis 部署边界错误")
	}
}

// TestKeyPendingMixedOldFlusher 验证旧 writer 与旧 flusher 同时存在时集合数量碰巧等于队列长度仍不会漏费。
// 前置专用 Redis；模拟旧消费者只移除队头、新写入继续发生，必须保持原子回退；夹具清理进程。
func TestKeyPendingMixedOldFlusher(t *testing.T) {
	c := testRedis(t)
	ctx := t.Context()
	row := SpendLog{RequestID: "indexed", KeyID: "other", APIKey: "other-hash", Spend: 2, SpendValid: true}
	if err := c.EnqueueSpend(row); err != nil {
		t.Fatal(err)
	}
	row.RequestID, row.KeyID, row.APIKey, row.Spend = "old-writer", "key", "old-hash", 7
	raw, _ := json.Marshal(row)
	if err := c.rdb.RPush(ctx, "xhub:spendlog", raw).Err(); err != nil {
		t.Fatal(err)
	}
	if err := c.rdb.LPop(ctx, "xhub:spendlog").Err(); err != nil {
		t.Fatal(err)
	}
	if c.rdb.SCard(ctx, "xhub:spend:key-indexed").Val() != c.rdb.LLen(ctx, "xhub:spendlog").Val() {
		t.Fatal("测试未形成数量相同的混合版本状态")
	}
	if got, err := c.HotKeySpend("key"); err != nil || got != 7 {
		t.Fatalf("旧消费者导致假覆盖: %v %v", got, err)
	}
	row.RequestID, row.Spend = "new-writer", 3
	if err := c.EnqueueSpend(row); err != nil {
		t.Fatal(err)
	}
	if got, err := c.HotKeySpend("key"); err != nil || got != 10 {
		t.Fatalf("新 writer 不应覆盖未知队列的快照: %v %v", got, err)
	}
	// 模拟旧消费者排空全部队列，随后新队列应丢弃遗留覆盖成员并忽略旧 Key 索引。
	if err := c.rdb.Del(ctx, "xhub:spendlog").Err(); err != nil {
		t.Fatal(err)
	}
	row.RequestID, row.KeyID, row.Spend = "after-empty", "other", 5
	if err := c.EnqueueSpend(row); err != nil {
		t.Fatal(err)
	}
	if got, err := c.HotKeySpend("other"); err != nil || got != 5 {
		t.Fatalf("排空后遗留索引误计费: %v %v", got, err)
	}
	if c.rdb.SCard(ctx, "xhub:spend:key-indexed").Val() != 1 {
		t.Fatal("空队列未重建覆盖集合")
	}
}
