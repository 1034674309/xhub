package regression

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/sunqirui1987/xhub/internal/config"
	"github.com/sunqirui1987/xhub/internal/gateway"
	"github.com/sunqirui1987/xhub/internal/gateway/billing"
)

// hardBudgetDeployment 声明本地供应商执行且测试可核对的完整输入/输出范围；不依赖外部模型目录。
// 参数为模型名，返回明确单价和 cap 能力的部署，无持久副作用。
func hardBudgetDeployment(model string) config.ModelEntry {
	dep := chatDeployment(model)
	dep.ModelInfo["max_input_tokens"] = 1024
	dep.ModelInfo["max_output_tokens"] = 8192
	dep.ModelInfo["supported_parameters"] = []string{"max_tokens"}
	return dep
}

// hardBudgetAmount 计算夹具公开的预占上界；参数为部署，返回金额，失败终止测试，不调用供应商。
func hardBudgetAmount(t *testing.T, dep config.ModelEntry) float64 {
	t.Helper()
	plan, _, err := billing.StrictBudgetPlan(dep, []byte(`{"messages":[{"role":"user","content":"Test budget."}]}`), "openai-chat", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return plan.Amount
}

// TestHardBudgetConcurrentInstances 验证八请求跨两个网关实例，只有一次可支付尝试真正访问上游。
// 前置私有 PostgreSQL、无 Redis 和可阻塞本地上游；持有第一请求时其余请求必须 429，最后清理服务/schema。
func TestHardBudgetConcurrentInstances(t *testing.T) {
	t.Setenv("XHUB_REGRESSION_REDIS_URL", "")
	const model = "hard-concurrent"
	dep := hardBudgetDeployment(model)
	h := newHarness(t, dep)
	admin := h.adminSession()
	c := h.openScope(t, admin, model)
	h.setKeyBudget(t, admin, c.key, hardBudgetAmount(t, dep))
	other := gateway.New(h.gw.Cfg, h.store, h.db)
	server := httptest.NewServer(other.Handler())
	defer server.Close()
	release := make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	h.mu.Lock()
	h.rules = map[string]upstreamRule{model: {hold: release}}
	h.mu.Unlock()
	results := make(chan reply, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			target := &harness{t: t, server: h.server}
			if i%2 == 1 {
				target.server = server
			}
			results <- target.do(http.MethodPost, "/v1/chat/completions", c.key, chatRequest(model, fmt.Sprintf("Concurrent request %d", i)))
		}(i)
	}
	for i := 0; i < 7; i++ {
		select {
		case got := <-results:
			if got.status != 429 {
				t.Fatalf("并发预占拒绝错误: %s", got.describe())
			}
		case <-time.After(20 * time.Second):
			t.Fatal("并发请求未及时被原子预占拒绝")
		}
	}
	if len(h.upstreamCalls()) != 1 {
		t.Fatalf("跨实例并发超额访问上游: %d", len(h.upstreamCalls()))
	}
	info := h.ok(http.MethodGet, "/key/info?key="+c.keyID, c.session, nil).json()["info"].(map[string]any)
	if info["budget_pending"].(float64) <= 0 {
		t.Fatalf("在途占用没有管理可见性: %v", info)
	}
	once.Do(func() { close(release) })
	wg.Wait()
	got := <-results
	if got.status != 200 {
		t.Fatalf("可支付调用失败: %s", got.describe())
	}
	if got.header("X-XHub-Max-Output-Tokens") != "4096" || got.header("X-XHub-Budget-Input-Bound") != "1024" {
		t.Fatalf("未公开实际输入/输出上界: %v", got)
	}
	if h.upstreamCalls()[0].Body["max_tokens"] != float64(4096) {
		t.Fatal("最终真实出站没有默认 cap")
	}
	holds, err := h.db.PendingKeyBudget(t.Context(), c.keyID)
	if err != nil || len(holds) != 0 {
		t.Fatalf("成功结算未释放: %+v %v", holds, err)
	}
}

// TestHardBudgetPendingRecovery 验证数据库事务失败后预占一直保留至完整账单成功恢复。
// 前置私有 PostgreSQL 和本地模型；仅给私有 request_logs 加 CHECK，finally 删除并由 harness 清理资源。
func TestHardBudgetPendingRecovery(t *testing.T) {
	t.Setenv("XHUB_REGRESSION_REDIS_URL", "")
	const model = "hard-pending"
	dep := hardBudgetDeployment(model)
	h := newHarness(t, dep)
	admin := h.adminSession()
	c := h.openScope(t, admin, model)
	h.setKeyBudget(t, admin, c.key, hardBudgetAmount(t, dep))
	if _, err := h.db.Engine.Exec("ALTER TABLE request_logs ADD CONSTRAINT hard_pending_failure CHECK (request_id = '')"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = h.db.Engine.Exec("ALTER TABLE request_logs DROP CONSTRAINT IF EXISTS hard_pending_failure")
	})
	h.ok(http.MethodPost, "/v1/chat/completions", c.key, chatRequest(model, "Retain budget until billing commits."))
	holds, err := h.db.PendingKeyBudget(t.Context(), c.keyID)
	if err != nil || len(holds) != 1 || holds[0].State != "settling" {
		t.Fatalf("失败事务提前释放或未标记占用: %+v %v", holds, err)
	}
	if got := h.do(http.MethodPost, "/v1/chat/completions", c.key, chatRequest(model, "Cannot reuse pending funds.")); got.status != 429 {
		t.Fatalf("待结算预算被重用: %s", got.describe())
	}
	// 管理调额必须保留已承诺项目费用，并返回可解释的400而非存储500。
	if got := h.do(http.MethodPost, "/project/update", admin, map[string]any{"project_id": c.projectID, "max_budget": 0}); got.status != 400 || got.json()["error"].(map[string]any)["type"] != "quota_allocation_exceeded" {
		t.Fatalf("待结算时项目降额未原子拒绝: %s", got.describe())
	}
	// 新实例不能把无法排空的旧账单队列当成零；503期间上游不应再被调用。
	other := gateway.New(h.gw.Cfg, h.store, h.db)
	restarted := httptest.NewServer(other.Handler())
	defer restarted.Close()
	probe := &harness{t: t, server: restarted}
	if got := probe.do(http.MethodPost, "/v1/chat/completions", c.key, chatRequest(model, "Reject pending queue during restart.")); got.status != 503 {
		t.Fatalf("启动账单闸门未拒绝: %s", got.describe())
	}
	if len(h.upstreamCalls()) != 1 {
		t.Fatalf("无法清算的重启实例仍访问上游: %d", len(h.upstreamCalls()))
	}
	if _, err := h.db.Engine.Exec("ALTER TABLE request_logs DROP CONSTRAINT hard_pending_failure"); err != nil {
		t.Fatal(err)
	}
	h.flushSpend()
	holds, err = h.db.PendingKeyBudget(t.Context(), c.keyID)
	if err != nil || len(holds) != 0 {
		t.Fatalf("恢复后占用未释放: %+v %v", holds, err)
	}
	key, err := h.db.GetKey(t.Context(), c.keyID)
	if err != nil || !nearlyEqual(key.Spend, expectedCost()) {
		t.Fatalf("恢复费用错误: %+v %v", key, err)
	}
}

// TestHardBudgetUnknownUsage 验证缺失或超出声明的上游用量不会退款，也不会静默扣超预算。
// 前置私有数据库和故意违约的本地供应商；管理详情显示 uncertain/原因，夹具清理未知占用。
func TestHardBudgetUnknownUsage(t *testing.T) {
	for _, usage := range []map[string]any{{"prompt_tokens": 2048, "completion_tokens": 5}, {"prompt_tokens": 11}, {"prompt_tokens": 11, "completion_tokens": 5, "prompt_tokens_details": map[string]any{"cached_tokens": 3}}} {
		t.Run(fmt.Sprint(usage), func(t *testing.T) {
			t.Setenv("XHUB_REGRESSION_REDIS_URL", "")
			const model = "hard-unknown"
			dep := hardBudgetDeployment(model)
			h := newHarness(t, dep)
			admin := h.adminSession()
			c := h.openScope(t, admin, model)
			h.setKeyBudget(t, admin, c.key, hardBudgetAmount(t, dep))
			h.mu.Lock()
			h.rules = map[string]upstreamRule{model: {usage: usage}}
			h.mu.Unlock()
			h.ok(http.MethodPost, "/v1/chat/completions", c.key, chatRequest(model, "Preserve uncertain billing."))
			holds, err := h.db.PendingKeyBudget(t.Context(), c.keyID)
			if err != nil || len(holds) != 1 || holds[0].State != "uncertain" || holds[0].Reason == "" {
				t.Fatalf("不确定费用占用丢失: %+v %v", holds, err)
			}
			key, err := h.db.GetKey(t.Context(), c.keyID)
			if err != nil || key.Spend != 0 {
				t.Fatalf("异常用量不应静默穿透硬额度: %+v %v", key, err)
			}
			if got := h.do(http.MethodPost, "/v1/chat/completions", c.key, chatRequest(model, "Cannot reuse uncertain funds.")); got.status != 429 {
				t.Fatalf("未知费用被提前退款: %s", got.describe())
			}
		})
	}
}

// TestHardBudgetFinalProtocolCaps 验证三协议标准/原生入口都在最终出站 JSON 写入真实默认输出上限。
// 前置每例私有 schema 和本地供应商；响应公开 cap，完整用量落账释放预占，不调用付费上游。
func TestHardBudgetFinalProtocolCaps(t *testing.T) {
	for _, tc := range []struct{ protocol, transport, path, native, cap string }{
		{"chat", "bypass_openai_chat", "/v1/chat/completions", "/bypass/openai/v1/chat/completions", "max_tokens"},
		{"responses", "bypass_openai_responses", "/v1/responses", "/bypass/openai/v1/responses", "max_output_tokens"},
		{"messages", "bypass_anthropic_messages", "/v1/messages", "/bypass/anthropic/v1/messages", "max_tokens"},
	} {
		for _, native := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/native=%t", tc.protocol, native), func(t *testing.T) {
				t.Setenv("XHUB_REGRESSION_REDIS_URL", "")
				dep := hardBudgetDeployment("hard-" + tc.protocol)
				dep.ModelInfo["transport"] = tc.transport
				dep.ModelInfo["endpoint_types"] = []string{tc.protocol, "bypass:" + map[string]string{"chat": "openai-chat", "responses": "openai-responses", "messages": "anthropic-messages"}[tc.protocol]}
				h := newHarness(t, dep)
				if tc.protocol == "responses" {
					h.scriptBody(dep.ModelName, []byte(`{"id":"resp_budget","object":"response","status":"completed","output":[{"id":"msg_budget","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"budget-ok","annotations":[]}]}],"usage":{"input_tokens":11,"output_tokens":5,"total_tokens":16}}`))
				}
				admin := h.adminSession()
				c := h.openScope(t, admin, "caps")
				h.setKeyBudget(t, admin, c.key, 1)
				body := chatRequest(dep.ModelName, "Verify default output limit.")
				if tc.protocol == "responses" {
					delete(body, "messages")
					body["input"] = "Verify default output limit."
				}
				path := tc.path
				if native {
					path = tc.native
				}
				got := h.ok(http.MethodPost, path, c.key, body)
				if got.header("X-XHub-Max-Output-Tokens") != "4096" {
					t.Fatalf("协议未公开输出 cap: %s", got.describe())
				}
				calls := h.upstreamCalls()
				if len(calls) != 1 || calls[0].Body[tc.cap] != float64(4096) {
					t.Fatalf("协议实际出站 cap 不一致: %+v", calls)
				}
				holds, err := h.db.PendingKeyBudget(t.Context(), c.keyID)
				if err != nil || len(holds) != 0 {
					t.Fatalf("正常完整 usage 没有释放占用: %+v %v", holds, err)
				}
			})
		}
	}
}

// TestHardBudgetCancellationRetainsAttempt 验证客户端取消已出站请求后不会把未知费用退款再使用。
// 前置私有数据库与阻塞上游；取消真实 HTTP 客户端，等待 handler 保存 uncertain 后释放夹具，清理所有资源。
func TestHardBudgetCancellationRetainsAttempt(t *testing.T) {
	t.Setenv("XHUB_REGRESSION_REDIS_URL", "")
	dep := hardBudgetDeployment("hard-cancel")
	h := newHarness(t, dep)
	admin := h.adminSession()
	c := h.openScope(t, admin, "cancel")
	h.setKeyBudget(t, admin, c.key, hardBudgetAmount(t, dep))
	release := h.holdUpstream(dep.ModelName)
	defer release()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	raw, _ := json.Marshal(chatRequest(dep.ModelName, "Cancel after submission."))
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, h.server.URL+"/v1/chat/completions", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")
	done := make(chan struct{})
	go func() {
		defer close(done)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()
		}
	}()
	deadline := time.Now().Add(10 * time.Second)
	for len(h.upstreamCalls()) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if len(h.upstreamCalls()) != 1 {
		t.Fatal("取消前没有真正出站")
	}
	cancel()
	<-done
	for {
		holds, err := h.db.PendingKeyBudget(t.Context(), c.keyID)
		if err != nil {
			t.Fatal(err)
		}
		if len(holds) == 1 && holds[0].State == "uncertain" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("取消后未知占用没有保留状态: %+v", holds)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := h.do(http.MethodPost, "/v1/chat/completions", c.key, chatRequest(dep.ModelName, "Cancelled request still may cost.")); got.status != 429 {
		t.Fatalf("取消导致预算重用: %s", got.describe())
	}
}

// TestHardBudgetRetryReservesSeparately 验证不确定失败的重试需要额外预算，而不是重用第一次占用。
// 前置私有数据库、本地返回 503 的上游和两次尝试路由；不确定费用保留，原请求错误/占用都可查询。
func TestHardBudgetRetryReservesSeparately(t *testing.T) {
	t.Setenv("XHUB_REGRESSION_REDIS_URL", "")
	dep := hardBudgetDeployment("hard-retry")
	h := newHarness(t, dep)
	admin := h.adminSession()
	c := h.openScope(t, admin, "retry")
	h.setKeyBudget(t, admin, c.key, hardBudgetAmount(t, dep))
	h.scriptStatus(dep.ModelName, 503)
	routeID := routeTemplate(t, h, admin, "budget retries", routeTemplateBody([]any{}, 2, 60, 0, 0))
	bindTemplate(t, h, admin, "key", c.keyID, routeID)
	got := h.do(http.MethodPost, "/v1/chat/completions", c.key, chatRequest(dep.ModelName, "Retry must reserve again."))
	if got.status != 429 {
		t.Fatalf("失败/预算拒绝契约错误: %s", got.describe())
	}
	if len(h.upstreamCalls()) != 1 {
		t.Fatalf("重试重复使用同一预算: %d", len(h.upstreamCalls()))
	}
	holds, err := h.db.PendingKeyBudget(t.Context(), c.keyID)
	if err != nil || len(holds) != 1 || holds[0].State != "uncertain" {
		t.Fatalf("失败尝试预算被丢弃: %+v %v", holds, err)
	}
}
