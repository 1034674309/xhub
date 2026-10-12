package affinity

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sunqirui1987/xhub/internal/auth"
	dpcontract "github.com/sunqirui1987/xhub/internal/dataplane/contract"
	llmdialogue "github.com/sunqirui1987/xhub/internal/llm/dialogue"
)

// TestSessionSignals 验证明确会话信号合并、严格错误、缓存键指纹和旧字段忽略。
// 参数 t 为测试上下文，无返回值；前置每次独立的内存 State 和 HTTP 夹具。
// 断言正常来源、冲突/空值/超长/控制字符拒绝、重复头冲突及无信号随机 ID 不合并。
// 不调用外部存储或上游，所有夹具随测试释放，不修改用户数据。
func TestSessionSignals(t *testing.T) {
	for _, tc := range []struct {
		name, header string
		body         map[string]any
		source       string
		invalid      bool
	}{
		{"header", "conv", map[string]any{"session_id": "conv"}, "explicit", false},
		{"conflict", "conv", map[string]any{"session_id": "other"}, "", true},
		{"numeric", "", map[string]any{"session_id": 1}, "", true},
		{"empty", "", map[string]any{"session_id": ""}, "", true},
		{"space", " bad", nil, "", true},
		{"control", "", map[string]any{"session_id": "bad\n"}, "", true},
		{"length", "", map[string]any{"session_id": strings.Repeat("a", 257)}, "", true},
		{"cache", "", map[string]any{"prompt_cache_key": "secret"}, "cache_key", false},
		{"legacy", "", map[string]any{"metadata": map[string]any{"session_id": "old", "user_id": "old"}}, "generated", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/v1/chat/completions", nil)
			if tc.header != "" {
				r.Header.Set("X-Session-Id", tc.header)
			}
			r.Header.Set("session-id", "old")
			p := (&State{}).PlanRoute(r, "a", tc.body, &auth.Principal{UserID: "u"})
			if (p.Err != nil) != tc.invalid || (!tc.invalid && (p.SessionID == "" || p.SessionSource != tc.source)) {
				t.Fatalf("信号解析错误: %+v", p)
			}
			if tc.source == "cache_key" && (!strings.HasPrefix(p.SessionID, "cache:v1:") || strings.Contains(p.SessionID, "secret")) {
				t.Fatal("缓存信号泄露明文")
			}
		})
	}
	r := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	r.Header.Add("X-Session-Id", "a")
	r.Header.Add("X-Session-Id", "b")
	if (&State{}).PlanRoute(r, "a", nil, nil).Err == nil {
		t.Fatal("重复头冲突未拒绝")
	}
	a := (&State{}).PlanRoute(nil, "a", nil, nil)
	b := (&State{}).PlanRoute(nil, "a", nil, nil)
	if a.SessionID == b.SessionID {
		t.Fatal("无信号请求被合并")
	}
}

// TestPromptFingerprint 验证首轮指纹完整性、追加历史稳定性及不能可靠推断时的空返回值。
// 参数 t 为测试上下文，无返回值；前置本地 JSON 结构，覆盖文本、媒体、协议指令和不可编码值。
// 断言模型/工具/后续轮次不改变指纹，媒体变化改变指纹，未知或不完整内容拒绝推断。
// 全部为纯内存数据，无外部依赖，无需持久化清理。
func TestPromptFingerprint(t *testing.T) {
	first := map[string]any{"messages": []any{map[string]any{"role": "system", "content": "Be helpful."}, map[string]any{"role": "user", "content": "Hello"}}}
	id := contentSessionID(first)
	if len(id) != len("prompt:v1:")+64 {
		t.Fatalf("指纹长度错误 %q", id)
	}
	next := map[string]any{"model": "b", "tools": []any{"changed"}, "messages": append(first["messages"].([]any), map[string]any{"role": "assistant", "content": "Hi"}, map[string]any{"role": "user", "content": "More"})}
	if contentSessionID(next) != id {
		t.Fatal("追加历史或工具/模型改变会话")
	}
	for _, b := range []map[string]any{
		{"messages": []any{map[string]any{"role": "system", "content": "Be helpful."}}},
		{"messages": []any{map[string]any{"role": "user", "content": " "}}},
		{"input": []any{map[string]any{"type": "function_call_output", "output": "ok"}}},
		{"messages": []any{map[string]any{"role": "user", "content": []any{map[string]any{"type": "unknown", "text": "Hello"}}}}},
	} {
		if contentSessionID(b) != "" {
			t.Fatal("不完整内容被推断", b)
		}
	}
	for _, b := range []map[string]any{{"input": "Hello", "instructions": "Be helpful."}, {"system": []any{map[string]any{"type": "text", "text": "Be helpful."}}, "messages": []any{map[string]any{"role": "user", "content": "Hello"}}}} {
		if contentSessionID(b) == "" {
			t.Fatal("协议内容未推断", b)
		}
	}
	// media 构造仅媒体 URL 不同的消息；参数 url，返回本地正文夹具，不访问实际图片。
	media := func(url string) map[string]any {
		return map[string]any{"messages": []any{map[string]any{"role": "user", "content": []any{map[string]any{"type": "text", "text": "Describe"}, map[string]any{"type": "image_url", "image_url": map[string]any{"url": url}}}}}}
	}
	if contentSessionID(media("image-a")) == contentSessionID(media("image-b")) || contentSessionID(media("")) != "" {
		t.Fatal("媒体内容被忽略")
	}
	if fingerprint("x:", func() {}) != "" {
		t.Fatal("不可序列化内容被接受")
	}
}

// TestContinuationPinsDoNotCrossCallerOrModel 验证父响应会话继承、冲突拒绝及隔离范围。
// 参数 t 为测试上下文，无返回值；先在独立内存 State 提交带历史的成功响应。
// 断言原范围继承目标与来源，其他调用方/模型/入口不能续接，并验证键字段边界不碰撞。
// 无真实上游或共享存储，测试结束释放全部内存，不影响用户响应记录。
func TestContinuationPinsDoNotCrossCallerOrModel(t *testing.T) {
	s := &State{}
	s.CommitRoute(dpcontract.RoutePlan{Alias: "a", Caller: "user:alice", Endpoint: "responses", SessionID: "conversation", History: []llmdialogue.Turn{{Role: "user", Text: "Hello"}}}, "dep-a", "resp")
	body := map[string]any{"previous_response_id": "resp"}
	r := httptest.NewRequest("POST", "/v1/responses", nil)
	p := s.PlanRoute(r, "a", body, &auth.Principal{UserID: "alice"})
	if p.Err != nil || p.Pinned != "dep-a" || p.SessionID != "conversation" || p.SessionSource != "parent" {
		t.Fatalf("继承失败 %+v", p)
	}
	body["session_id"] = "other"
	if s.PlanRoute(r, "a", body, &auth.Principal{UserID: "alice"}).Err == nil {
		t.Fatal("冲突未拒绝")
	}
	delete(body, "session_id")
	for _, tc := range []struct{ alias, user, path string }{{"a", "bob", "/v1/responses"}, {"b", "alice", "/v1/responses"}, {"a", "alice", "/v1/chat/completions"}} {
		if s.PlanRoute(httptest.NewRequest("POST", tc.path, nil), tc.alias, body, &auth.Principal{UserID: tc.user}).Err == nil {
			t.Fatal("跨范围归属泄漏")
		}
	}
	if sessionPinKey("a|b", "c", "d") == sessionPinKey("a", "b|c", "d") {
		t.Fatal("键分隔符冲突")
	}
}

// TestConcurrentClaimAndLateCommit 验证首次并发申请收敛及旧请求无法覆盖新偏好。
// 参数 t 为测试上下文，无返回值；前置共享于本测试的独立 State，32 个请求各持私有计划。
// 等待所有协程结束后验证唯一赢家、迟到提交保留新目标、过期清理及本机容量限制。
// 所有缓存和通道仅在本测试使用，协程通过 WaitGroup 收尾，无外部数据需要清理。
func TestConcurrentClaimAndLateCommit(t *testing.T) {
	s := &State{}
	var wg sync.WaitGroup
	winners := make(chan string, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p := s.PlanRoute(httptest.NewRequest("POST", "/v1/chat/completions", nil), "a", map[string]any{"session_id": "shared"}, &auth.Principal{UserID: "u"})
			winners <- p.Claim(dpcontract.RouteTarget{DeploymentID: fmt.Sprint(i), Model: "a", Pool: "a"}).DeploymentID
		}(i)
	}
	wg.Wait()
	close(winners)
	winner := ""
	for got := range winners {
		if winner == "" {
			winner = got
		}
		if winner != got {
			t.Fatal("并发首次申请未收敛")
		}
	}
	// plan 为同一已鉴权会话读取独立计划；无参数，返回私有快照和回调，用于模拟请求先后完成。
	plan := func() dpcontract.RoutePlan {
		return s.PlanRoute(httptest.NewRequest("POST", "/v1/chat/completions", nil), "a", map[string]any{"session_id": "shared"}, &auth.Principal{UserID: "u"})
	}
	old, newer := plan(), plan()
	s.CommitRoute(newer, "new", "")
	s.CommitRoute(old, "late", "")
	if plan().Pinned != "new" {
		t.Fatal("迟到提交覆盖新偏好")
	}
	s.affinitySetFor(sessionPinKey("a", "user:u", "shared", "chat"), "expired", -time.Second)
	if plan().Pinned != "" {
		t.Fatal("过期偏好未清理")
	}
	s.affinityMu.Lock()
	s.affinity = map[string]affinityPin{}
	for i := 0; i < localCapacity; i++ {
		s.affinity[fmt.Sprint(i)] = affinityPin{deployment: "x", until: time.Now().Add(time.Hour)}
	}
	s.affinityMu.Unlock()
	s.affinitySet("last", "x")
	if len(s.affinity) > localCapacity {
		t.Fatal("本机缓存无界")
	}
}
