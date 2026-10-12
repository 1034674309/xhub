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
