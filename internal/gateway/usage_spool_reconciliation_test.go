package gateway

import (
	"errors"
	"github.com/sunqirui1987/xhub/cmd/regression/testsupport"
	"github.com/sunqirui1987/xhub/internal/gateway/usagespool"
	"github.com/sunqirui1987/xhub/internal/iam"
	"math"
	"path/filepath"
	"testing"
	"time"
)

// TestUsageSpoolReconciledFence 独立验证真实PG人工终态优先于严格指纹：匹配费用确认、不匹配保留、旧账单无指纹仍尊重已审计核销、不能释放另一尝试；所有数据位于私有schema/临时目录并自动清理。
func TestUsageSpoolReconciledFence(t *testing.T) {
	for _, kind := range []string{"matching", "mismatch", "original", "legacy_matching", "other_hold", "no_charge"} {
		t.Run(kind, func(t *testing.T) {
			ctx := t.Context()
			db, err := iam.Open(ctx, testsupport.Postgres(t, "spool_reconciled"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { db.Close() })
			if _, err = db.EnsureAdmin(ctx, "review@example.com", "Review", "password123"); err != nil {
				t.Fatal(err)
			}
			admin, err := db.UserByEmail(ctx, "review@example.com")
			if err != nil {
				t.Fatal(err)
			}
			key, _, err := db.CreateKey(ctx, iam.Actor{}, iam.KeyInput{OwnerType: iam.OwnerPersonal, UserID: admin.ID})
			if err != nil {
				t.Fatal(err)
			}
			original := iam.UsageRecord{RequestID: "review-call", TS: time.Now().UTC(), KeyID: key.ID, UserID: admin.ID, Model: "synthetic", OwnerType: iam.OwnerPersonal, Status: "error", Cost: .2, PromptTokens: 3}
			for _, id := range []string{"hold-one", "hold-two"} {
				if _, err = db.ReserveBudget(ctx, id, "key", key.ID, 1, iam.BudgetReservationOptions{CallID: original.RequestID, StartedAt: original.TS}); err != nil {
					t.Fatal(err)
				}
				if err = db.SetBudgetReservationState(ctx, id, "uncertain", "review fixture"); err != nil {
					t.Fatal(err)
				}
			}
			if err = db.RecordUsage(ctx, []iam.UsageRecord{original}); err != nil {
				t.Fatal(err)
			}
			if kind == "legacy_matching" {
				if _, err = db.Engine.Exec("UPDATE usage_events SET usage_fingerprint='' WHERE request_id=?", original.RequestID); err != nil {
					t.Fatal(err)
				}
			}
			rows, _, err := db.ListBudgetReservations(ctx, key.ID, "uncertain", 50, 0)
			if err != nil {
				t.Fatal(err)
			}
			fingerprint := ""
			for _, r := range rows {
				if r.ReservationID == "hold-one" {
					fingerprint = r.Fingerprint
				}
			}
			cost, outcome := .4, "charged"
			if kind == "no_charge" {
				cost, outcome = 0, "no_charge"
			}
			_, _, err = db.ReconcileBudgetReservation(ctx, iam.Actor{ID: admin.ID, Kind: "session"}, "hold-one", iam.BudgetReconciliationInput{Outcome: outcome, VerifiedCost: cost, Evidence: "Independent synthetic invoice", ExpectedFingerprint: fingerprint})
			if err != nil {
				t.Fatal(err)
			}
			sp, err := usagespool.Open(usagespool.Config{Dir: filepath.Join(t.TempDir(), "spool")})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { sp.Close() })
			late := original
			late.BudgetReservationID = "hold-one"
			late.Cost = cost
			if kind == "mismatch" {
				late.Cost = .5
			}
			if kind == "original" {
				late = original
			}
			if kind == "other_hold" {
				late = original
				late.BudgetReservationID = "hold-two"
			}
			if err = sp.Put(late); err != nil {
				t.Fatal(err)
			}
			s := &Server{IAM: db, usageSpoolEnabled: true, usageSpool: sp}
			flushErr := s.FlushLocalUsage(ctx)
			status, _ := sp.Status()
			wantPending := 0
			if kind == "mismatch" || kind == "other_hold" {
				wantPending = 1
			}
			if status.Pending != wantPending || (wantPending == 0 && flushErr != nil) || (wantPending == 1 && !errors.Is(flushErr, iam.ErrConflict)) {
				t.Fatalf("fence状态错误 pending=%d want=%d err=%v", status.Pending, wantPending, flushErr)
			}
			var event iam.UsageEvent
			if _, err = db.Engine.Where("request_id=?", original.RequestID).Get(&event); err != nil || math.Abs(event.Cost-(.2+cost)) > 1e-12 || event.PromptTokens != 3 {
				t.Fatalf("费用/Token发生重复或覆盖 cost=%v err=%v", event.Cost, err)
			}
			holds, err := db.PendingKeyBudget(ctx, key.ID)
			if err != nil || len(holds) != 1 || holds[0].RequestID != "hold-two" {
				t.Fatalf("其他尝试预占被释放 holds=%+v err=%v", holds, err)
			}
		})
	}
}
