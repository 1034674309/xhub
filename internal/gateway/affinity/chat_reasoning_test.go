package affinity

import (
	"net/http/httptest"
	"testing"

	"github.com/sunqirui1987/xhub/internal/auth"
	dpcontract "github.com/sunqirui1987/xhub/internal/dataplane/contract"
)

// TestChatReasoningOwnedSession 验证 Chat 推理扩展只复用已成功会话的原连接，普通空扩展不触发固定归属。
// 参数 t 为上下文；内存 State 覆盖有效续轮、跨身份/模型/会话及非法扩展，无数据库或上游调用，无需清理。
func TestChatReasoningOwnedSession(t *testing.T) {
	s := &State{}
	p := &auth.Principal{UserID: "alice"}
	r := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	r.Header.Set("X-Session-Id", "conversation")
	body := map[string]any{"messages": []any{map[string]any{"role": "user", "content": "Read the file."}}}
	first := s.PlanRoute(r, "model", body, p)
	first.Target = dpcontract.RouteTarget{DeploymentID: "dep", ConnectionID: "account", Model: "model", Pool: "model"}
	s.CommitRoute(first, "dep", "resp")
	message := map[string]any{"role": "assistant", "content": "", "reasoning_items": []any{map[string]any{"type": "reasoning", "id": "rs", "encrypted_content": "cipher"}}}
	body["messages"] = []any{message}
	got := s.PlanRoute(r, "model", body, p)
	if got.Err != nil || !got.Required || got.Target.ConnectionID != "account" {
		t.Fatalf("Chat 扩展未固定原连接: %+v", got)
	}
	for _, scope := range []struct{ user, model, session string }{{"bob", "model", "conversation"}, {"alice", "other", "conversation"}, {"alice", "model", "other"}} {
		req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
		req.Header.Set("X-Session-Id", scope.session)
		if got := s.PlanRoute(req, scope.model, body, &auth.Principal{UserID: scope.user}); got.Err == nil {
			t.Fatalf("Chat 扩展跨范围重放被接受: %+v", scope)
		}
	}
	for _, value := range []any{nil, []any{}} {
		message["reasoning_items"] = value
		if requiresNativeOwnership(body) {
			t.Fatal("空 Chat 扩展不应固定归属")
		}
	}
	message["reasoning_items"] = "malformed"
	if !requiresNativeOwnership(body) {
		t.Fatal("非法 Chat 扩展不能作为普通请求回退")
	}
}
