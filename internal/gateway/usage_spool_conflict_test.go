package gateway

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/sunqirui1987/xhub/internal/gateway/usagespool"
	"github.com/sunqirui1987/xhub/internal/iam"
)

// TestUsageSpoolExactAcknowledgement 验证PG同ID另一份快照不能触发磁盘确认，覆盖正常即时提交与后台重放两个实际入口。
// 前置私有PG/临时磁盘，pending和已入账费用冲突必须ErrConflict且保留证据；相同完整快照可幂等确认，结束清理。
func TestUsageSpoolExactAcknowledgement(t *testing.T) {
	for _, source := range []string{"pending", "ledger", "matching"} {
		for _, replay := range []bool{false, true} {
			t.Run(source+map[bool]string{false: "/persist", true: "/replay"}[replay], func(t *testing.T) {
				db := testIdentityStore(t)
				sp, err := usagespool.Open(usagespool.Config{Dir: filepath.Join(t.TempDir(), "spool")})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = sp.Close() })
				record := iam.UsageRecord{RequestID: "same-id", TS: time.Now().UTC(), Model: "synthetic", Status: "success", Cost: 1}
				old := record
				if source != "matching" {
					old.Cost = 0
				}
				if source == "pending" {
					raw, _ := json.Marshal(old)
					if _, err := db.Engine.Exec("INSERT INTO usage_pending(request_id,payload) VALUES (?,?::jsonb)", old.RequestID, string(raw)); err != nil {
						t.Fatal(err)
					}
				} else if err := db.RecordUsage(t.Context(), []iam.UsageRecord{old}); err != nil {
					t.Fatal(err)
				}
				s := &Server{IAM: db, usageSpoolEnabled: true, usageSpool: sp}
				if replay {
					if err = sp.Put(record); err != nil {
						t.Fatal(err)
					}
					err = s.FlushLocalUsage(t.Context())
				} else {
					_, err = s.persistUsageSnapshot(record)
				}
				state, _ := sp.Status()
				if source == "matching" {
					if err != nil || state.Pending != 0 {
						t.Fatalf("正常重放未确认: pending=%d err=%v", state.Pending, err)
					}
				} else {
					if !errors.Is(err, iam.ErrConflict) || state.Pending != 1 {
						t.Fatalf("另一份账单误确认磁盘证据: pending=%d err=%v", state.Pending, err)
					}
				}
			})
		}
	}
}
