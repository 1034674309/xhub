package gateway

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sunqirui1987/xhub/internal/iam"
)

// TestArchiveScheduling 验证真实数据库配置热更新、到期消费、轮次上限和停用后的积压处理。
// 前置为隔离 schema 与内存对象边界；覆盖 manifest 部分失败及幂等重试，夹具清理数据库，无真实云写入。
func TestArchiveScheduling(t *testing.T) {
	cfg, st, db := testGatewayStores(t)
	s := New(cfg, st, db)
	ctx := context.Background()
	last := time.Unix(100000, 0)
	mem := &memoryArchive{objects: map[string][]byte{}}
	factory := func(c *iam.ArchiveConfig) (archiveObjectStore, error) {
		if c.Prefix != "billing" {
			t.Fatal("积压事件没有使用原目标", c)
		}
		return mem, nil
	}
	if got, err := s.runArchiveCycle(ctx, last.Add(time.Hour), last, factory); err == nil || !got.Equal(last) {
		t.Fatal("缺少配置推进调度", got, err)
	}
	config := iam.ArchiveConfig{Bucket: "test", Region: "z0", Prefix: "billing", Credentials: "test-only", Enabled: true}
	if err := db.SaveArchiveConfiguration(ctx, testActor, config); err != nil {
		t.Fatal("内部调用默认化失败", err)
	}
	saved, err := db.ArchiveConfiguration(ctx, 0)
	if err != nil || saved.IntervalSeconds != 2 || saved.BatchSize != 20 {
		t.Fatal("内部调用没有保存默认调度", saved, err)
	}
	config.IntervalSeconds, config.BatchSize = 300, 1
	if err := db.SaveArchiveConfiguration(ctx, testActor, config); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"schedule-one", "schedule-two", "schedule-three"} {
		if err := db.RecordUsage(ctx, []iam.UsageRecord{{RequestID: id, Model: "test", Status: "success"}}); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := s.runArchiveCycle(ctx, last.Add(299*time.Second), last, factory); err != nil || !got.Equal(last) || len(mem.objects) != 0 {
		t.Fatal("到期前提前归档", got, err)
	}
	got, err := s.runArchiveCycle(ctx, last.Add(300*time.Second), last, factory)
	if err != nil || !got.Equal(last.Add(300*time.Second)) || len(mem.objects) != 2 {
		t.Fatal("到期轮次未遵守单事件上限", got, err, len(mem.objects))
	}
	last = got
	// 修改最新配置即影响下一轮调度；停用后新账务不入队，但旧队列仍写其原版本目标。
	config.IntervalSeconds, config.BatchSize, config.Enabled, config.Prefix = 2, 2, false, "changed"
	if err := db.SaveArchiveConfiguration(ctx, testActor, config); err != nil {
		t.Fatal(err)
	}
	if err := db.RecordUsage(ctx, []iam.UsageRecord{{RequestID: "schedule-disabled", Model: "test", Status: "success"}}); err != nil {
		t.Fatal(err)
	}
	counts, _, err := db.ArchiveStatus(ctx)
	if err != nil || counts["pending"] != 2 {
		t.Fatal("停用后仍捕获新账务", counts, err)
	}
	mem.failManifest = true
	got, err = s.runArchiveCycle(ctx, last.Add(2*time.Second), last, factory)
	if err == nil || !got.Equal(last.Add(2*time.Second)) || len(mem.objects) != 3 {
		t.Fatal("manifest 部分失败没有停止轮次", got, err, len(mem.objects))
	}
	counts, _, err = db.ArchiveStatus(ctx)
	if err != nil || counts["failed"] != 1 || counts["pending"] != 1 || counts["done"] != 1 {
		t.Fatal("部分上传被错误确认成功", counts, err)
	}
	last = got
	if got, err := s.runArchiveCycle(ctx, last.Add(time.Second), last, factory); err != nil || !got.Equal(last) {
		t.Fatal("失败后绕过配置间隔", got, err)
	}
	if err := db.RetryArchive(ctx, testActor); err != nil {
		t.Fatal(err)
	}
	mem.failManifest = false
	got, err = s.runArchiveCycle(ctx, last.Add(2*time.Second), last, factory)
	counts, _, statusErr := db.ArchiveStatus(ctx)
	if err != nil || statusErr != nil || counts["done"] != 3 || counts["pending"] != 0 || counts["failed"] != 0 || len(mem.objects) != 6 {
		t.Fatal("部分失败重试不幂等或积压未排空", counts, err, statusErr, len(mem.objects))
	}
	if next, err := s.runArchiveCycle(ctx, got.Add(2*time.Second), got, factory); err != nil || !next.Equal(got.Add(2*time.Second)) || len(mem.objects) != 6 {
		t.Fatal("空队列重复写对象", next, err)
	}
	config.IntervalSeconds = 1
	before, err := db.ArchiveConfiguration(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveArchiveConfiguration(ctx, testActor, config); !errors.Is(err, iam.ErrInvalid) {
		t.Fatal("内部非法调度未拒绝", err)
	}
	after, err := db.ArchiveConfiguration(ctx, 0)
	if err != nil || after.Revision != before.Revision {
		t.Fatal("内部非法保存新增版本", after, err)
	}
}
