package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/qiniu/go-sdk/v7/storage"

	"github.com/sunqirui1987/xhub/internal/iam"
)

// TestKodoLiveArchive 前置显式云测试授权及专用 bucket/AK/SK/region；验证真实 SDK 数据、manifest 上传和冲突保护。
// 默认跳过而不偷用开发凭据；启用后缺配置立即失败，独立随机前缀与私有 schema，结束删除本轮所有对象。
func TestKodoLiveArchive(t *testing.T) {
	if os.Getenv("XHUB_KODO_LIVE") != "1" {
		t.Skip("真实七牛测试未启用：需要 XHUB_KODO_LIVE=1 和专用测试空间凭据")
	}
	values := map[string]string{}
	for _, name := range []string{"BUCKET", "REGION", "AK", "SK"} {
		values[name] = os.Getenv("XHUB_KODO_TEST_" + name)
		if values[name] == "" {
			t.Fatalf("缺少 XHUB_KODO_TEST_%s", name)
		}
	}
	cfg, st, db := testGatewayStores(t)
	cfg.GeneralSettings.MasterKey = strings.Repeat("l", 32)
	plain, _ := json.Marshal(map[string]string{"AK": values["AK"], "SK": values["SK"]})
	secret, err := enterpriseSecret(cfg.GeneralSettings.MasterKey, string(plain), false)
	if err != nil {
		t.Fatal(err)
	}
	unique, err := enterpriseRandom()
	if err != nil {
		t.Fatal(err)
	}
	c := iam.ArchiveConfig{Bucket: values["BUCKET"], Region: values["REGION"], Prefix: "xhub-tests/" + unique, Credentials: secret, Enabled: true}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := db.SaveArchiveConfiguration(ctx, testActor, c); err != nil {
		t.Fatal(err)
	}
	if err := db.RecordUsage(ctx, []iam.UsageRecord{{RequestID: unique, Model: "live-archive-test", Status: "success"}}); err != nil {
		t.Fatal(err)
	}
	store, err := newKodoArchive(&c, cfg.GeneralSettings.MasterKey)
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{}
	// 在每次网络上传之前登记键，确保响应丢失或中途失败也会清理已创建对象。
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 45*time.Second)
		defer stop()
		for _, key := range keys {
			if err := store.Delete(cleanup, key); err != nil {
				t.Errorf("云端清理失败，对象 %s: %v", key, err)
			}
		}
	})
	s := New(cfg, st, db)
	_, events, err := db.ArchiveStatus(ctx)
	if err != nil || len(events) != 1 {
		t.Fatal("归档事件未入队", err)
	}
	saved, err := db.ArchiveConfiguration(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	event := events[0]
	// 使用与消费者相同的不可变对象命名，提前登记 data 和 manifest。
	key := fmt.Sprintf("%s/v%d/events/%020d.jsonl.gz", c.Prefix, saved.Revision, event.ID)
	keys = append(keys, key, key+".manifest.json")
	ok, err := s.processArchive(ctx, func(target *iam.ArchiveConfig) (archiveObjectStore, error) {
		return newKodoArchive(target, cfg.GeneralSettings.MasterKey)
	})
	if err != nil || !ok {
		t.Fatal("真实云端归档失败", err)
	}
	counts, events, err := db.ArchiveStatus(ctx)
	if err != nil || counts["done"] != 1 || events[0].ObjectKey != key || events[0].SHA256 == "" {
		t.Fatal("归档未在上传完成后确认", counts, err)
	}
	// 独立查询云端数据与 manifest，不能只依赖本地 outbox 的 done 状态作为云验证证据。
	k := store.(*kodoArchive)
	manager := storage.NewBucketManagerEx(k.mac, k.cfg, k.client)
	stat, err := manager.Stat(k.bucket, key)
	if err != nil || stat.Fsize != events[0].ByteSize {
		t.Fatal("云端归档数据不存在或长度错误", err)
	}
	manifest, _ := json.Marshal(map[string]any{"schema_version": 1, "event_id": event.ID, "object_key": key, "sha256": events[0].SHA256, "byte_size": events[0].ByteSize, "row_count": 1, "configuration_revision": saved.Revision})
	stat, err = manager.Stat(k.bucket, key+".manifest.json")
	if err != nil || stat.Hash != kodoContentHash(manifest) || stat.Fsize != int64(len(manifest)) {
		t.Fatal("云端 manifest 内容不匹配", err)
	}
	if err := store.Put(ctx, key, []byte("conflicting content")); err == nil {
		t.Fatal("云端不可变对象被覆盖")
	}
}
