package regression

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestSessionAffinityConcurrentLateSuccess 验证真实网关和 Redis 中重叠首次请求、故障切换及迟到成功的比较更新。
// 前置专属 Redis、私有 PostgreSQL 和本地上游；旧 alpha 成功必须不能覆盖新 beta，第三轮保持 beta。
// 同时检查三条成功日志、显式会话及五级费用；释放阻塞请求后由 harness 清理 schema、连接和子进程。
func TestSessionAffinityConcurrentLateSuccess(t *testing.T) {
	isolatedSpendRedis(t)
	const model = "concurrent-session-model"
	h := newHarness(t, deployment(model, "late-alpha", nil), deployment(model, "late-beta", nil))
	if h.gw.Live == nil {
		t.Fatal("并发契约必须连接专属 Redis")
	}
	admin := h.adminSession()
	owner := h.openScope(t, admin, "concurrent-affinity")
	template := routeTemplate(t, h, admin, "concurrent affinity", routeTemplateBody([]any{}, 1, 30, 0, 0))
	bindTemplate(t, h, admin, "key", owner.keyID, template)
	// setWeights 为本场景调整模型默认分配；参数 beta 为备用部署权重，无返回值。
	// 通过真实管理接口保存，非法配置由 h.ok 报错；首次排除 beta，第二轮恢复它以验证临时候选优先。
	// 仅修改本测试私有模型，harness 结束删除 schema。
	setWeights := func(beta int) {
		t.Helper()
		h.ok("PUT", "/model/default", admin, map[string]any{"model_name": model, "weights": map[string]any{"allocations": []any{
			map[string]any{"deployment_id": "late-alpha", "weight": 1}, map[string]any{"deployment_id": "late-beta", "weight": beta},
		}}})
	}
	setWeights(0)
	before := h.moneyOf(t, owner)
	release := h.holdUpstream("late-alpha")
	defer release()
	// 在后台仅收集结果，不调用带 Fatal 的测试辅助函数；主测试负责有界等待和业务断言。
	raw, err := json.Marshal(chatRequest(model, "First pending round"))
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, h.server.URL+"/v1/chat/completions", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+owner.key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-Id", "concurrent-session")
	type outcome struct {
		status  int
		headers http.Header
		body    []byte
		err     error
	}
	done := make(chan outcome, 1)
	go func() {
		client := &http.Client{Timeout: 15 * time.Second}
		response, err := client.Do(req)
		if err != nil {
			done <- outcome{err: err}
			return
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		done <- outcome{status: response.StatusCode, headers: response.Header, body: body, err: err}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for len(h.upstreamCalls()) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("首次请求未到达 alpha")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// 首请求已经复制了正常阻塞规则；只让后续 alpha 请求立即失败，明确制造迟到成功。
	setWeights(1)
	h.scriptStatus("late-alpha", 500)
	h.mu.Lock()
	rule := h.rules["late-alpha"]
	rule.hold = nil
	h.rules["late-alpha"] = rule
	h.mu.Unlock()
	next := h.doHeaders("POST", "/v1/chat/completions", owner.key, chatRequest(model, "Second switches target"), map[string]string{"X-Session-Id": "concurrent-session"})
	if next.status != 200 {
		t.Fatalf("第二轮未成功切换: %s", next.describe())
	}
	if got := strings.Join(h.upstreamSince(0), ","); got != "late-alpha,late-alpha,late-beta" {
		t.Fatalf("临时候选及回退顺序错误: %s", got)
	}
	release()
	var old outcome
	select {
	case old = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("迟到请求未完成")
	}
	if old.err != nil || old.status != 200 || old.headers.Get("X-Session-Id") != "concurrent-session" {
		t.Fatalf("旧请求未成功保持会话: %+v", old)
	}
	// 恢复 alpha 资格后第三轮仍应选 beta，避免用失败或冷却掩盖错误覆盖。
	h.scriptStatus("late-alpha", 0)
	last := h.doHeaders("POST", "/v1/chat/completions", owner.key, chatRequest(model, "Third keeps new target"), map[string]string{"X-Session-Id": "concurrent-session"})
	if last.status != 200 || last.header("X-Session-Id") != "concurrent-session" {
		t.Fatalf("第三轮会话失败: %s", last.describe())
	}
	if got := strings.Join(h.upstreamSince(0), ","); got != "late-alpha,late-alpha,late-beta,late-beta" {
		t.Fatalf("迟到成功覆盖新偏好: %s", got)
	}
	h.flushSpend()
	cost := parseFloatOrZero(old.headers.Get("x-litellm-response-cost")) + parseFloatOrZero(next.header("x-litellm-response-cost")) + parseFloatOrZero(last.header("x-litellm-response-cost"))
	if cost <= 0 || !h.moneyOf(t, owner).grewBy(before, cost) {
		t.Fatal("并发与失败尝试没有按三次成功唯一结算")
	}
	rows := h.successRows(t, admin, model)
	if len(rows) != 3 {
		t.Fatalf("并发成功日志数量=%d，预期3", len(rows))
	}
	for _, row := range rows {
		detail := h.ok("GET", "/spend/logs/ui/"+stringField(row, "request_id"), admin, nil).json()
		if detail["session_id"] != "concurrent-session" || detail["session_source"] != "explicit" || detail["model"] != model {
			t.Fatalf("并发日志归属错误: %v", detail)
		}
	}
}
