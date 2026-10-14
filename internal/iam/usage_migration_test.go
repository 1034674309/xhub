package iam

import (
	"testing"

	"github.com/sunqirui1987/xhub/cmd/regression/testsupport"
)

// TestUsageSessionSourceUpgrade 验证空旧库及有历史账单的旧库在 Open 启动时自动补列。
// 前置专属 schema，删除新字段模拟旧结构；验证历史来源为空、新来源可写、非空约束与重复迁移不重复计费。
// 无外部上游依赖；连接由本测试关闭，schema 由 testsupport 在结束时清理。
func TestUsageSessionSourceUpgrade(t *testing.T) {
	for _, history := range []bool{false, true} {
		name := "空旧库"
		if history {
			name = "已有历史账单"
		}
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()
			dsn := testsupport.Postgres(t, "usage_upgrade")
			db, err := Open(ctx, dsn)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			old := usageRecord("historical", "", 1.25)
			if history {
				if err := db.RecordUsage(ctx, []UsageRecord{old}); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := db.Engine.Exec("ALTER TABLE usage_events DROP COLUMN session_source"); err != nil {
				t.Fatal(err)
			}
			// 走真实启动入口，而非测试自行补列，防止只验证新建表或手工修复。
			reopened, err := Open(ctx, dsn)
			if err != nil {
				t.Fatalf("旧库启动迁移失败: %v", err)
			}
			defer reopened.Close()
			if history {
				event, err := reopened.GetUsageEvent(ctx, UsageQuery{}, old.RequestID)
				if err != nil || event == nil || event.SessionSource != "" || event.Cost != old.Cost {
					t.Fatalf("历史账单或未知会话来源未保留: event=%+v err=%v", event, err)
				}
			}
			current := usageRecord("after-upgrade", "", 2.5)
			current.SessionSource = "explicit"
			if err := reopened.RecordUsage(ctx, []UsageRecord{current}); err != nil {
				t.Fatalf("升级后不能写入新账单: %v", err)
			}
			for range 2 {
				if err := reopened.Migrate(ctx); err != nil {
					t.Fatal(err)
				}
				if err := reopened.RecordUsage(ctx, []UsageRecord{current}); err != nil {
					t.Fatal(err)
				}
			}
			event, err := reopened.GetUsageEvent(ctx, UsageQuery{}, current.RequestID)
			if err != nil || event == nil || event.SessionSource != "explicit" || countEvents(t, reopened, current.RequestID) != 1 {
				t.Fatalf("重复迁移或重放改变了账单: event=%+v err=%v", event, err)
			}
			if _, err := reopened.Engine.Exec("UPDATE usage_events SET session_source = NULL WHERE request_id = ?", current.RequestID); err == nil {
				t.Fatal("升级后的会话来源字段丢失 NOT NULL 约束")
			}
			wantRequests, wantCost := int64(1), current.Cost
			if history {
				wantRequests++
				wantCost += old.Cost
			}
			if dailyRequests(t, reopened, "") != wantRequests || dailyCost(t, reopened, "") != wantCost {
				t.Fatal("重复迁移或重放导致日用量重复累计")
			}
		})
	}
}
