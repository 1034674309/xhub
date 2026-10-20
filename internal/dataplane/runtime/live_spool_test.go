package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/sunqirui1987/xhub/cmd/regression/testsupport"
	"github.com/sunqirui1987/xhub/internal/iam"
	"github.com/sunqirui1987/xhub/internal/live"
)

// spoolRuntime 提供Flush最小宿主；未使用能力保留嵌入接口，便于发现意外调用；记录磁盘hook次数。
type spoolRuntime struct {
	Runtime
	db    *iam.DB
	err   error
	calls int
}

// Identity 返回隔离数据库或nil，供Flush判断是否可落账；无副作用。
func (s *spoolRuntime) Identity() *iam.DB { return s.db }

// Redis 返回nil，验证磁盘恢复不依赖Redis；无副作用。
func (s *spoolRuntime) Redis() *live.Client { return nil }

// FlushLocalUsage 记录恢复调用并返回注入错误，不访问外部上游或修改账单。
func (s *spoolRuntime) FlushLocalUsage(context.Context) error { s.calls++; return s.err }

// TestFlushLocalSpoolHook 验证无Redis时调用磁盘恢复hook，失败不panic，无身份库时不调用；私有PG/schema自动清理。
func TestFlushLocalSpoolHook(t *testing.T) {
	db, err := iam.Open(t.Context(), testsupport.Postgres(t, "runtime_spool"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, tc := range []struct {
		name  string
		db    *iam.DB
		err   error
		calls int
	}{{"success", db, nil, 1}, {"retained_failure", db, errors.New("retained"), 1}, {"no_identity", nil, nil, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			h := &spoolRuntime{db: tc.db, err: tc.err}
			Flush(h)
			if h.calls != tc.calls {
				t.Fatalf("恢复调用次数=%d，预期%d", h.calls, tc.calls)
			}
		})
	}
}
