package regression

import (
	"context"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sunqirui1987/xhub/internal/live"
)

// isolatedSpendRedis 为结算回归启动专用 Redis，并让当前 harness 使用其随机端口。
// 参数 t 注册连接与进程清理；返回原生客户端用于可控故障注入。缺少二进制时失败，不跳过可靠计费验证。
// 不访问已有 Redis，测试退出恢复环境变量、关闭客户端并终止临时进程。
func isolatedSpendRedis(t *testing.T) *redis.Client {
	t.Helper()
	binary, err := exec.LookPath("redis-server")
	if err != nil {
		t.Fatalf("结算回归需要本地 redis-server: %v", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	_, port, _ := net.SplitHostPort(addr)
	_ = listener.Close()
	cmd := exec.Command(binary, "--bind", "127.0.0.1", "--port", port, "--save", "", "--appendonly", "no")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	client := redis.NewClient(&redis.Options{Addr: addr, MaxRetries: -1, DialTimeout: time.Second})
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for client.Ping(ctx).Err() != nil {
		if ctx.Err() != nil {
			t.Fatal("临时 Redis 未就绪")
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Setenv("XHUB_REGRESSION_REDIS_URL", "redis://"+addr+"/0")
	return client
}

// TestSpendQueueFailureRecovery 验证真实 HTTP 推理经过唯一可靠队列，数据库失败和提交后确认失败均可安全恢复。
// 前置隔离 PostgreSQL schema、专用 Redis 和本地协议上游；五层计数/日志仅结算一次，重复入队和刷写无副作用。
// 数据库故障只给当前请求增加临时 CHECK，Redis 故障只破坏专用计数；恢复故障并由 harness/夹具清理全部资源。
func TestSpendQueueFailureRecovery(t *testing.T) {
	for _, failure := range []string{"数据库提交失败", "Redis确认失败"} {
		t.Run(failure, func(t *testing.T) {
			rdb := isolatedSpendRedis(t)
			const model = "regression-current-spend-queue"
			h := newHarness(t, chatDeployment(model))
			admin := h.adminSession()
			c := h.openScope(t, admin, "current-spend-queue")
			if h.gw.Live == nil {
				t.Fatal("网关未连接专用 Redis，回归不能降级为空跑")
			}
			before := h.moneyOf(t, c)
			r := h.ok(http.MethodPost, "/v1/chat/completions", c.key, chatRequest(model, "Verify reliable accounting recovery"))
			cost := parseFloatOrZero(r.header("x-litellm-response-cost"))
			rows, raw := h.gw.Live.PeekLogs(10)
			if cost <= 0 || len(rows) != 1 || len(raw) != 1 || rows[0].RequestID != r.header("x-litellm-call-id") {
				t.Fatalf("HTTP 调用未生成唯一可计费队列记录: cost=%v rows=%+v", cost, rows)
			}
			row := rows[0]
			refs := []string{live.SpendRef("key", row.APIKey), live.SpendRef("user", c.userID), live.SpendRef("team", c.teamID), live.SpendRef("org", c.orgID), live.SpendRef("project", c.projectID)}
			for _, ref := range refs {
				if got := h.gw.Live.HotSpend(ref); !nearlyEqual(got, cost) {
					t.Fatalf("%s 未记录待结算费用: %v，预期 %v", ref, got, cost)
				}
			}
			if failure == "数据库提交失败" {
				// PostgreSQL DDL 不接受绑定参数；按 SQL 字符串规则转义本例调用 ID，只拒绝本事件。
				// 在用量事件已插入、正文写入失败时证明整个事务回滚，不能只模拟事务开始前失败。
				statement := "ALTER TABLE request_logs ADD CONSTRAINT queue_test_failure CHECK (request_id <> '" + strings.ReplaceAll(row.RequestID, "'", "''") + "')"
				if _, err := h.db.Engine.Exec(statement); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					_, _ = h.db.Engine.Exec("ALTER TABLE request_logs DROP CONSTRAINT IF EXISTS queue_test_failure")
				})
			} else {
				if err := rdb.Set(t.Context(), "xhub:spend:"+refs[1], "broken-counter", 0).Err(); err != nil {
					t.Fatal(err)
				}
			}
			h.flushSpend()
			if pending, pendingRaw := h.gw.Live.PeekLogs(10); len(pending) != 1 || len(pendingRaw) != 1 || pendingRaw[0] != raw[0] {
				t.Fatalf("故障后丢失待重试日志: %+v", pending)
			}
			if got := h.gw.Live.HotSpend(refs[0]); !nearlyEqual(got, cost) {
				t.Fatalf("故障后错误释放密钥热花费: %v", got)
			}
			user, err := h.db.GetUser(t.Context(), c.userID)
			if err != nil {
				t.Fatal(err)
			}
			if failure == "数据库提交失败" {
				if !nearlyEqual(user.Spend, before.user) {
					t.Fatalf("数据库事务失败仍扣费: %v", user.Spend)
				}
				if detail := h.do(http.MethodGet, "/spend/logs/ui/"+row.RequestID, admin, nil); detail.status != http.StatusNotFound {
					t.Fatalf("数据库事务失败仍发布日志: status=%d body=%s", detail.status, detail.body)
				}
				if _, err := h.db.Engine.Exec("ALTER TABLE request_logs DROP CONSTRAINT queue_test_failure"); err != nil {
					t.Fatal(err)
				}
			} else {
				if !nearlyEqual(user.Spend-before.user, cost) {
					t.Fatalf("确认失败前数据库应已提交一次: %v", user.Spend)
				}
				if err := rdb.Set(t.Context(), "xhub:spend:"+refs[1], cost, 0).Err(); err != nil {
					t.Fatal(err)
				}
			}
			if err := h.gw.Live.EnqueueSpend(row); err != nil {
				t.Fatal(err)
			}
			h.flushSpend()
			h.flushSpend()
			if after := h.moneyOf(t, c); !after.grewBy(before, cost) {
				t.Fatalf("恢复后五层费用不是唯一一次: before=%+v after=%+v cost=%v", before, after, cost)
			}
			if logs := h.successRows(t, admin, model); len(logs) != 1 {
				t.Fatalf("恢复后日志数量=%d，预期 1", len(logs))
			}
			if err := h.gw.Live.EnqueueSpend(row); err != nil {
				t.Fatal(err)
			}
			if pending, _ := h.gw.Live.PeekLogs(10); len(pending) != 0 {
				t.Fatal("确认后重放重新入队")
			}
			for _, ref := range refs {
				if got := h.gw.Live.HotSpend(ref); !nearlyEqual(got, 0) {
					t.Fatalf("恢复后 %s 热花费未释放: %v", ref, got)
				}
			}
			if got := rdb.Exists(t.Context(), "xhub:spend:ids").Val(); got != 0 {
				t.Fatal("当前业务仍写入废弃聚合索引")
			}
		})
	}
}
