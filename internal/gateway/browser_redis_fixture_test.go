package gateway

import (
	"context"
	"net"
	"os/exec"
	"testing"
	"time"

	"github.com/sunqirui1987/xhub/internal/iam"
	"github.com/sunqirui1987/xhub/internal/live"
)

// browserPrivateRedis 为浏览器夹具创建随机端口的独占 Redis；参数 t 注册清理，返回本机 URL。
// 仅测试二进制调用，缺少程序或启动失败立即报错；不读取外部 Redis 配置，不清空已有缓存。
// 子进程关闭持久化，测试结束终止进程并回收等待状态，所有测试数据随进程消失。
func browserPrivateRedis(t *testing.T) string {
	t.Helper()
	binary, err := exec.LookPath("redis-server")
	if err != nil {
		t.Fatalf("Redis 浏览器验收需要本地 redis-server: %v", err)
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
	url := "redis://" + addr + "/0"
	for deadline := time.Now().Add(5 * time.Second); ; {
		client, err := live.Open(url)
		if err == nil {
			_ = client.Close()
			return url
		}
		if time.Now().After(deadline) {
			t.Fatalf("专属 Redis 未就绪: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// browserSpendWorker 为隔离夹具定时调用真实 FlushSpend；参数 s 是已连接专属 Redis 的网关。
// 返回停止并等待清理的函数；只改变测试等待时长，不绕过队列、事务或确认语义。
// 调用方须在关闭 Redis 和数据库前停止 worker；最后一次刷写保存尾部事件。
func browserSpendWorker(s *Server) func() {
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.FlushSpend()
			case <-stop:
				s.FlushSpend()
				return
			}
		}
	}()
	return func() { close(stop); <-done }
}

// TestBrowserRedisFixtureIsolationAndFlush 验证浏览器夹具只访问自建 Redis，真实刷写保留来源且错误事件不确认。
// 前置本地 redis-server 和私有 PostgreSQL schema；覆盖空队列、正常及失败事件，停止 worker 后检查尾部数据。
// 连接、子进程和 schema 由夹具清理；不修改共享缓存或用户数据。
func TestBrowserRedisFixtureIsolationAndFlush(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		t.Run(map[bool]string{false: "正常及空队列", true: "失败不确认"}[invalid], func(t *testing.T) {
			_, _, db := testGatewayStores(t)
			url := browserPrivateRedis(t)
			client, err := live.Open(url)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			other, err := live.Open(browserPrivateRedis(t))
			if err != nil {
				t.Fatal(err)
			}
			defer other.Close()
			if err := client.WriteAffinity(t.Context(), "fixture-isolation", "private", time.Minute); err != nil {
				t.Fatal(err)
			}
			if value, err := other.ReadAffinity(t.Context(), "fixture-isolation"); err != nil || value != "" {
				t.Fatalf("两个浏览器 Redis 共享了状态: value=%s err=%v", value, err)
			}
			s := &Server{IAM: db, Live: client}
			// 空队列正常停止；随后两事件走同一真实队列，其中失败分支必须完整保留待重试数据。
			browserSpendWorker(s)()
			if invalid {
				// 真实数据库拒绝本夹具事件，避免入口参数校验提前阻止事件入队而没有验证刷写失败。
				if _, err := db.Engine.Exec("ALTER TABLE usage_events ADD CONSTRAINT browser_fixture_reject CHECK (request_id <> 'explicit')"); err != nil {
					t.Fatal(err)
				}
			}
			for _, source := range []string{"explicit", "parent"} {
				row := live.SpendLog{RequestID: source, Model: "fixture-requested-group", SessionID: "fixture-session", SessionSource: source, Status: "success", Start: time.Now().Format(time.RFC3339Nano), End: time.Now().Format(time.RFC3339Nano)}
				if err := client.EnqueueSpend(row); err != nil {
					t.Fatal(err)
				}
			}
			browserSpendWorker(s)()
			var rows []iam.UsageEvent
			if err := db.Engine.Context(context.Background()).Find(&rows); err != nil {
				t.Fatal(err)
			}
			pending, _ := client.PeekLogs(10)
			if invalid {
				if len(rows) != 0 || len(pending) != 2 {
					t.Fatalf("错误刷写应整批不落库且保留队列: persisted=%+v pending=%+v", rows, pending)
				}
				return
			}
			if len(rows) != 2 || len(pending) != 0 {
				t.Fatalf("停止 worker 应刷写尾部事件并确认: persisted=%+v pending=%+v", rows, pending)
			}
			for _, row := range rows {
				if row.SessionSource != row.RequestID || row.Model != "fixture-requested-group" || row.SessionID != "fixture-session" {
					t.Fatalf("夹具刷写丢失请求模型或会话归属: %+v", row)
				}
			}
		})
	}
}
