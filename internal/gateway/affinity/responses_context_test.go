package affinity

import (
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/sunqirui1987/xhub/internal/auth"
	dpcontract "github.com/sunqirui1987/xhub/internal/dataplane/contract"
	llmdialogue "github.com/sunqirui1987/xhub/internal/llm/dialogue"
)

// TestResponsesContextIsolationAndExpiry 验证续接历史往返、工具字段保留及范围隔离。
// 参数 t 为测试上下文，无返回值；前置独立内存 State，发布带工具调用的成功历史和归属。
// 断言其他身份/模型/入口无法读取，损坏或过期历史拒绝续接，过期记录被清除。
// 不依赖 Redis 或供应商，测试结束释放内存夹具，不访问或覆盖用户历史。
func TestResponsesContextIsolationAndExpiry(t *testing.T) {
	s := &State{}
	history := []llmdialogue.Turn{{Role: "user", Text: "first"}, {Role: "assistant", Calls: []llmdialogue.Turn{{ID: "call", Name: "lookup", Arguments: "{}"}}}}
	plan := dpcontract.RoutePlan{Alias: "a", Caller: "user:alice", Endpoint: "responses", SessionID: "conversation", History: history}
	s.CommitRoute(plan, "dep", "chatcmpl-id")
	body := map[string]any{"previous_response_id": "chatcmpl-id"}
	req := httptest.NewRequest("POST", "/responses", nil)
	got := s.PlanRoute(req, "a", body, &auth.Principal{UserID: "alice"})
	if got.Err != nil || got.Pinned != "dep" || !reflect.DeepEqual(got.History, history) {
		t.Fatalf("历史往返失败: %+v", got)
	}
	for _, tc := range []struct{ path, alias, user string }{{"/responses", "a", "bob"}, {"/responses", "b", "alice"}, {"/chat/completions", "a", "alice"}} {
		other := s.PlanRoute(httptest.NewRequest("POST", tc.path, nil), tc.alias, body, &auth.Principal{UserID: tc.user})
		if other.Err == nil || len(other.History) > 0 {
			t.Fatalf("跨范围泄露历史: %+v", other)
		}
	}
	key := responseContextKey("a", "user:alice", "chatcmpl-id", "responses")
	s.affinitySetFor(key, "invalid json", time.Hour)
	if s.PlanRoute(req, "a", body, &auth.Principal{UserID: "alice"}).Err == nil {
		t.Fatal("坏历史被接受")
	}
	s.affinitySetFor(key, "[]", -time.Second)
	if s.PlanRoute(req, "a", body, &auth.Principal{UserID: "alice"}).Err == nil || s.affinityGet(key) != "" {
		t.Fatal("过期历史被接受或未清理")
	}
}

// TestEncryptedHistoryRequiresOwnedDeployment 验证完整加密历史必须复用本身份已知部署，不能跨用户/别名/会话重选。
// 参数 t 为上下文；前置独立内存状态，正常文本可自由选路，密文首次/错误身份拒绝；内存数据随测试回收。
func TestEncryptedHistoryRequiresOwnedDeployment(t *testing.T) {
	state := &State{}
	principal := &auth.Principal{UserID: "alice"}
	request := httptest.NewRequest("POST", "/v1/responses", nil)
	request.Header.Set("X-Session-Id", "native-session")
	first := state.PlanRoute(request, "a", map[string]any{"input": "Hello"}, principal)
	if first.Err != nil || first.Required {
		t.Fatalf("普通首轮错误: %+v", first)
	}
	first.Target.DeploymentID = "native-deployment"
	state.CommitRoute(first, "native-deployment", "resp-1")
	body := map[string]any{"input": []any{map[string]any{"type": "reasoning", "encrypted_content": "cipher"}, map[string]any{"role": "user", "content": "Next"}}}
	next := state.PlanRoute(request, "a", body, principal)
	if next.Err != nil || !next.Required || next.Pinned != "native-deployment" {
		t.Fatalf("密文未固定原部署: %+v", next)
	}
	for _, tc := range []struct{ alias, user, session string }{{"b", "alice", "native-session"}, {"a", "bob", "native-session"}, {"a", "alice", "new-session"}} {
		req := httptest.NewRequest("POST", "/v1/responses", nil)
		req.Header.Set("X-Session-Id", tc.session)
		if got := state.PlanRoute(req, tc.alias, body, &auth.Principal{UserID: tc.user}); got.Err == nil {
			t.Fatalf("非归属密文被放行: %+v", tc)
		}
	}
	if requiresNativeOwnership(nil) || requiresNativeOwnership(map[string]any{"input": "Hello"}) {
		t.Fatal("纯文本误判为密文")
	}
}
