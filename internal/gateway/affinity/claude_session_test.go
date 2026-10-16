package affinity

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sunqirui1987/xhub/internal/auth"
	dpcontract "github.com/sunqirui1987/xhub/internal/dataplane/contract"
)

// TestClaudeSessionSignals 验证原生头、新版 JSON metadata、旧复合串及格式边界，不将普通用户身份当会话。
// 参数 t 为上下文；独立内存 State/HTTP 请求，验证优先级、非法与冲突头、无输入修改和非 Messages 隔离；无需外部清理。
func TestClaudeSessionSignals(t *testing.T) {
	for _, tc := range []struct {
		name, header, expected string
		user                   any
		invalid                bool
	}{
		{"header", "conversation-a", "conversation-a", nil, false},
		{"json", "", "conversation-a", `{"device_id":"device","account_uuid":"account","session_id":"conversation-a"}`, false},
		{"legacy", "", "conversation-a", "user_device_account__session_conversation-a", false},
		{"header-wins", "header-session", "header-session", `{"session_id":"metadata-session"}`, false},
		{"plain-user", "", "", "person-identifier", false},
		{"plain-session-marker", "", "", "person_session_identifier", false},
		{"invalid-json", "", "", `{"session_id":`, false},
		{"wrong-type", "", "", `{"session_id":42}`, false},
		{"invalid-value", "", "", `{"session_id":"with spaces"}`, false},
		{"legacy-empty", "", "", "user_device_account__session_", false},
		{"long-value", "", "", `{"session_id":"` + strings.Repeat("a", 257) + `"}`, false},
		{"oversized-metadata", "", "", `{"session_id":"conversation-a","device_id":"` + strings.Repeat("a", 4096) + `"}`, false},
		{"bad-header", "with spaces", "", nil, true},
		{"long-header", strings.Repeat("a", 257), "", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/v1/messages", nil)
			if tc.header != "" {
				r.Header.Set("X-Claude-Code-Session-Id", tc.header)
			}
			body := map[string]any{"metadata": map[string]any{"user_id": tc.user}}
			id, err := claudeSessionID(r, body)
			if (err != nil) != tc.invalid || id != tc.expected {
				t.Fatalf("Claude 会话解析错误: id=%q err=%v", id, err)
			}
			if body["metadata"].(map[string]any)["user_id"] != tc.user {
				t.Fatal("会话识别修改了客户端 metadata")
			}
			for _, path := range []string{"/v1/chat/completions", "/v1/responses"} {
				r.URL.Path = path
				plan := (&State{}).PlanRoute(r, "a", body, &auth.Principal{UserID: "alice"})
				if plan.Err != nil || plan.SessionSource != "generated" {
					t.Fatalf("Claude 会话信号影响了其他入口: %s %+v", path, plan)
				}
			}
		})
	}
	r := httptest.NewRequest("POST", "/v1/messages", nil)
	r.Header.Add("X-Claude-Code-Session-Id", "a")
	r.Header.Add("X-Claude-Code-Session-Id", "b")
	if _, err := claudeSessionID(r, nil); err == nil {
		t.Fatal("冲突的原生会话头未拒绝")
	}
	r.Header.Set("X-Session-Id", "gateway-session")
	plan := (&State{}).PlanRoute(r, "a", nil, &auth.Principal{UserID: "alice"})
	if plan.Err != nil || plan.SessionID != "gateway-session" {
		t.Fatalf("网关明确会话未优先: %+v", plan)
	}
}

// TestClaudeSessionRetainsEncryptedOwner 验证首条用户块改变后仍复用原生会话归属，并隔离并行会话/身份/模型/入口。
// 参数 t 为上下文；前置内存成功归属，续轮含不透明封装，要求固定原连接；无 HTTP 上游或持久化副作用。
func TestClaudeSessionRetainsEncryptedOwner(t *testing.T) {
	s := &State{}
	r := httptest.NewRequest("POST", "/v1/messages", nil)
	r.Header.Set("X-Claude-Code-Session-Id", "conversation-a")
	p := &auth.Principal{UserID: "alice"}
	body := map[string]any{"messages": []any{map[string]any{"role": "user", "content": "Read the file."}}}
	first := s.PlanRoute(r, "model", body, p)
	first.Target = dpcontract.RouteTarget{DeploymentID: "dep", ConnectionID: "account-a", Model: "model", Pool: "model"}
	s.CommitRoute(first, "dep", "response-a")
	body["messages"] = []any{map[string]any{"role": "user", "content": []any{map[string]any{"type": "text", "text": "Read the file.", "cache_control": map[string]any{"type": "ephemeral"}}}},
		map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "redacted_thinking", "data": "xhub_responses_reasoning:cipher"}}}}
	got := s.PlanRoute(r, "model", body, p)
	if got.Err != nil || !got.Required || got.Target.ConnectionID != "account-a" || got.SessionID != "conversation-a" || got.SessionSource != "explicit" {
		t.Fatalf("工具续轮丢失 Claude 会话归属: %+v", got)
	}
	for _, tc := range []struct{ user, alias, session, path string }{
		{"bob", "model", "conversation-a", "/v1/messages"}, {"alice", "other", "conversation-a", "/v1/messages"},
		{"alice", "model", "conversation-b", "/v1/messages"}, {"alice", "model", "conversation-a", "/v1/responses"},
	} {
		req := httptest.NewRequest("POST", tc.path, nil)
		req.Header.Set("X-Claude-Code-Session-Id", tc.session)
		if got := s.PlanRoute(req, tc.alias, body, &auth.Principal{UserID: tc.user}); got.Err == nil {
			t.Fatalf("跨范围密文归属被放行: %+v", tc)
		}
	}
}
