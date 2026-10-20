package iam

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// TestUsageExactSnapshotConfirmation 验证精确恢复接受相同快照、拒绝同ID金额/身份/正文冲突，并保留待结算原文。
// 前置私有PG，覆盖PG pending及既有账单两处证据；断言ErrConflict而非任意数据库错误，schema自动清理。
func TestUsageExactSnapshotConfirmation(t *testing.T) {
	for _, kind := range []string{"matching", "cost", "owner", "body", "pending", "legacy"} {
		t.Run(kind, func(t *testing.T) {
			db := testDB(t)
			original := UsageRecord{RequestID: "exact-" + kind, TS: time.Now().UTC(), Model: "synthetic", Cost: 1, RequestBody: `{"input":"Synthetic input"}`}
			submitted := original
			switch kind {
			case "cost", "pending":
				submitted.Cost = 2
			case "owner":
				submitted.UserID = "another-user"
			case "body":
				submitted.RequestBody = `{"input":"Different evidence"}`
			}
			if kind == "pending" {
				raw, _ := json.Marshal(original)
				if _, err := db.Engine.Exec("INSERT INTO usage_pending(request_id,payload) VALUES (?,?::jsonb)", original.RequestID, string(raw)); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := db.RecordUsage(t.Context(), []UsageRecord{original}); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "legacy" {
				if _, err := db.Engine.Exec("UPDATE usage_events SET usage_fingerprint='' WHERE request_id=?", original.RequestID); err != nil {
					t.Fatal(err)
				}
			}
			err := db.RecordUsageExact(t.Context(), []UsageRecord{submitted})
			if kind == "matching" {
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.Is(err, ErrConflict) {
				t.Fatalf("不同快照未精确拒绝: %v", err)
			}
			if kind == "pending" {
				durable, err := db.RecordUsageDurably(t.Context(), submitted)
				if durable || !errors.Is(err, ErrConflict) {
					t.Fatalf("另一份PG快照冒充当前快照持久化: durable=%v err=%v", durable, err)
				}
				if countEvents(t, db, original.RequestID) != 0 || pendingCount(t, db) != 1 {
					t.Fatal("冲突改变原账单证据")
				}
			}
		})
	}
}

// TestUsageFingerprintStartupMigration 验证既有表启动补列且重复启动幂等，历史事实保持原样、未知指纹不伪造。
// 前置私有PG先建一条真实账单再删除新列模拟旧库，重跑迁移两次；全部数据由私有schema清理。
func TestUsageFingerprintStartupMigration(t *testing.T) {
	db := testDB(t)
	record := UsageRecord{RequestID: "legacy-fingerprint", TS: time.Now().UTC(), Model: "synthetic", Cost: .25}
	if err := db.RecordUsage(t.Context(), []UsageRecord{record}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Engine.Exec("ALTER TABLE usage_events DROP COLUMN usage_fingerprint"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := db.Migrate(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	var event UsageEvent
	found, err := db.Engine.Where("request_id=?", record.RequestID).Get(&event)
	if err != nil || !found || event.Cost != .25 || event.UsageFingerprint != "" {
		t.Fatalf("迁移改写历史: found=%v cost=%v fingerprint=%q err=%v", found, event.Cost, event.UsageFingerprint, err)
	}
	if err := db.RecordUsageExact(t.Context(), []UsageRecord{record}); !errors.Is(err, ErrConflict) {
		t.Fatalf("旧无指纹账单误确认: %v", err)
	}
}
