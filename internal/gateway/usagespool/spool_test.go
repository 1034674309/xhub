package usagespool

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sunqirui1987/xhub/internal/iam"
)

// spoolRecord 创建带预占/冻结费率/正文的完整用量夹具，参数id唯一标识；不读取真实用户或外部凭据。
func spoolRecord(id string) iam.UsageRecord {
	return iam.UsageRecord{RequestID: id, BudgetReservationID: "hold-" + id, TS: time.Unix(100, 123).UTC(), EndedAt: time.Unix(101, 123).UTC(), Model: "test", UserID: "test-user", Cost: .0123, PromptTokens: 12, CompletionTokens: 3, RequestBody: `{"input":"Synthetic request"}`, ResponseBody: `{"output":"Synthetic reply"}`, PriceSnapshot: `{"frozen":true}`}
}

// openSpool 创建临时独占队列并注册关闭清理；参数t用于失败定位，config覆盖默认目录/容量，不产生持久外部数据。
func openSpool(t *testing.T, config Config) *Spool {
	t.Helper()
	if config.Dir == "" {
		config.Dir = filepath.Join(t.TempDir(), "spool")
	}
	s, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestSpoolRestartAndIdempotence 验证完整快照在关闭重开后恢复，重复Put/Replay不重复提交，权限严格0700/0600。
// 前置临时磁盘，检查reservation/价格/时间/正文逐字段相等及确认后清空；测试结束关闭目录并自动删除。
func TestSpoolRestartAndIdempotence(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "spool")
	s := openSpool(t, Config{Dir: dir})
	record := spoolRecord("../not-a-path")
	if err := s.Reserve(record.RequestID); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := s.Put(record); err != nil {
			t.Fatal(err)
		}
	}
	info, err := os.Stat(dir)
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatal("目录权限不是0700")
	}
	info, err = os.Stat(filepath.Join(dir, basename(record.RequestID)))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("快照权限不是0600")
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s = openSpool(t, Config{Dir: dir})
	committed := 0
	commit := func(_ context.Context, got iam.UsageRecord) error {
		committed++
		if !reflect.DeepEqual(got, record) {
			t.Fatalf("恢复载荷丢字段: %+v", got)
		}
		return nil
	}
	for range 2 {
		if _, err = s.Replay(t.Context(), 100, commit); err != nil {
			t.Fatal(err)
		}
	}
	status, err := s.Status()
	if err != nil || status.Pending != 0 || committed != 1 {
		t.Fatalf("重复恢复或残留: %+v commits=%d err=%v", status, committed, err)
	}
}

// TestSpoolConcurrentAdmission 验证在途请求按最大记录尺寸保守预留，多个并发准入不会超过容量。
// 前置仅容纳两个最大记录的临时队列，32个并发申请只允许两个；释放后可复用，无外部资源。
func TestSpoolConcurrentAdmission(t *testing.T) {
	s := openSpool(t, Config{MaxEntries: 2, MaxBytes: 8192, MaxRecordBytes: 4096})
	var accepted atomic.Int64
	var wg sync.WaitGroup
	for i := range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if s.Reserve(string(rune('a'+i))) == nil {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()
	if accepted.Load() != 2 {
		t.Fatalf("并发容量失守: %d", accepted.Load())
	}
	s.mu.Lock()
	for id := range s.reserved {
		delete(s.reserved, id)
	}
	s.mu.Unlock()
	if err := s.Reserve("next"); err != nil {
		t.Fatal("释放后不能复用", err)
	}
	if err := s.Reserve("next"); err != nil {
		t.Fatal("重复准入多占容量", err)
	}
	status, _ := s.Status()
	if status.Reserved != 1 || status.ReservedBytes != 4096 {
		t.Fatalf("重复预留: %+v", status)
	}
}

// TestSpoolSafeDirectoryAndLock 验证拒绝共享目录、最终目录符号链接和错误应用目录，不移动/更改用户文件。
// 前置临时文件系统，检查进程锁及数据完整性；全部句柄与路径由测试清理。
func TestSpoolSafeDirectoryAndLock(t *testing.T) {
	s := openSpool(t, Config{})
	if second, err := Open(s.config); err == nil {
		second.Close()
		t.Fatal("两个实例持有同一spool")
	}
	base := t.TempDir()
	target := filepath.Join(base, "real")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if other, err := Open(Config{Dir: link}); err == nil {
		other.Close()
		t.Fatal("接受符号链接目录")
	}
	unrelated := filepath.Join(base, "unrelated")
	os.Mkdir(unrelated, 0755)
	os.WriteFile(filepath.Join(unrelated, "user.txt"), []byte("keep"), 0644)
	if other, err := Open(Config{Dir: unrelated}); err == nil {
		other.Close()
		t.Fatal("接受非专用目录")
	}
	content, err := os.ReadFile(filepath.Join(unrelated, "user.txt"))
	if err != nil || string(content) != "keep" {
		t.Fatal("修改其他目录用户文件")
	}
	info, _ := os.Stat(unrelated)
	if info.Mode().Perm() != 0755 {
		t.Fatal("改变其他目录权限")
	}
}

// TestSpoolRecoveryAndCorruption 验证完整临时记录恢复、坏校验/版本/身份/符号链接隔离，坏证据阻止新付费准入。
// 前置为模拟崩溃遗留文件，任何情况不读取外部链接目标；有效记录仍可恢复，临时目录自动清理。
func TestSpoolRecoveryAndCorruption(t *testing.T) {
	for _, mode := range []string{"temporary", "checksum", "version", "identity", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "spool")
			s := openSpool(t, Config{Dir: dir})
			record := spoolRecord("recover")
			raw, _ := encode(record)
			s.Close()
			name := basename(record.RequestID)
			switch mode {
			case "temporary":
				name = ".pending-crashed.tmp"
			case "checksum":
				raw = append(raw[:len(raw)-1], []byte(`,"checksum":"wrong"}`)...)
			case "version":
				var value map[string]any
				json.Unmarshal(raw, &value)
				value["version"] = 99
				raw, _ = json.Marshal(value)
			case "identity":
				name = basename("other")
			}
			if mode == "symlink" {
				outside := filepath.Join(t.TempDir(), "private")
				os.WriteFile(outside, []byte("do not read"), 0600)
				if err := os.Symlink(outside, filepath.Join(dir, name)); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(filepath.Join(dir, name), raw, 0600); err != nil {
				t.Fatal(err)
			}
			s = openSpool(t, Config{Dir: dir})
			status, _ := s.Status()
			if mode == "temporary" {
				if status.Pending != 1 || status.Quarantined != 0 {
					t.Fatalf("完整临时记录没有恢复: %+v", status)
				}
			} else {
				if status.Quarantined != 1 {
					t.Fatalf("坏记录未隔离: %+v", status)
				}
				if !errors.Is(s.Reserve("next"), ErrCorrupt) {
					t.Fatal("坏账单证据未阻止准入")
				}
			}
		})
	}
}

// TestSpoolFailureRetentionAndBackoff 验证数据库冲突保留原文且不阻塞其他有效记录，重复Flush不会忙重试同一失败。
// 前置两份快照，提交器模拟一份冲突一份成功；只确认成功记录，测试关闭并删除目录。
func TestSpoolFailureRetentionAndBackoff(t *testing.T) {
	s := openSpool(t, Config{})
	for _, id := range []string{"bad", "good"} {
		if err := s.Put(spoolRecord(id)); err != nil {
			t.Fatal(err)
		}
	}
	attempts := map[string]int{}
	commit := func(_ context.Context, r iam.UsageRecord) error {
		attempts[r.RequestID]++
		if r.RequestID == "bad" {
			return iam.ErrConflict
		}
		return nil
	}
	n, err := s.Replay(t.Context(), 10, commit)
	if n != 1 || err == nil {
		t.Fatalf("失败阻塞其他账单: %d %v", n, err)
	}
	n, err = s.Replay(t.Context(), 10, commit)
	if n != 0 || err != nil || attempts["bad"] != 1 {
		t.Fatalf("失败忙重试: %d %v %+v", n, err, attempts)
	}
	status, _ := s.Status()
	if status.Pending != 1 {
		t.Fatal("失败记录被删除")
	}
}

// TestSpoolLimitsPermissionsAndConflict 验证超大/非法记录拒绝、不安全目录权限拒绝、重复身份不同内容保留冲突证据。
// 前置临时磁盘和合成请求，不截断正文或覆盖先前快照；测试清理全部路径。
func TestSpoolLimitsPermissionsAndConflict(t *testing.T) {
	s := openSpool(t, Config{MaxBytes: 8192, MaxRecordBytes: 4096, MaxEntries: 4})
	record := spoolRecord("same")
	if err := s.Put(record); err != nil {
		t.Fatal(err)
	}
	changed := record
	changed.Cost = .99
	if !errors.Is(s.Put(changed), ErrConflict) {
		t.Fatal("相同身份不同费用被覆盖")
	}
	status, _ := s.Status()
	if status.Pending != 1 || status.Quarantined != 1 {
		t.Fatalf("冲突证据未保留: %+v", status)
	}
	if err := s.Acknowledge(changed); !errors.Is(err, ErrConflict) {
		t.Fatal("错误快照确认删除原账单")
	}
	oversized := spoolRecord("oversized")
	oversized.RequestBody = strings.Repeat("x", 5000)
	if !errors.Is(s.Put(oversized), ErrFull) {
		t.Fatal("超大正文被截断/接受")
	}
	if err := s.Put(iam.UsageRecord{}); err == nil {
		t.Fatal("空记录被接受")
	}
	clean := openSpool(t, Config{})
	os.Chmod(clean.config.Dir, 0500)
	if err := clean.Reserve("denied"); err == nil {
		t.Fatal("只读目录准入未拒绝")
	}
	os.Chmod(clean.config.Dir, 0700)
}
