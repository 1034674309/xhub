package affinity

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sunqirui1987/xhub/internal/auth"
	dpcontract "github.com/sunqirui1987/xhub/internal/dataplane/contract"
	llmdialogue "github.com/sunqirui1987/xhub/internal/llm/dialogue"
)

// signedHistoryFixture 构造已经通过原生校验的合成签名/工具回复，供归属单元测试复用。
// 参数t报告夹具错误，返回回复；只有公开合成文本和假签名，不使用任何真实用户数据。
func signedHistoryFixture(t *testing.T) llmdialogue.DialogueResult {
	t.Helper()
	r, err := llmdialogue.ParseDialogueResult("anthropic-messages", []byte(`{"id":"msg_signed","type":"message","role":"assistant","content":[{"type":"thinking","thinking":"Synthetic thought","signature":"test-signature"},{"type":"tool_use","id":"call_1","name":"lookup","input":{}}],"stop_reason":"tool_use","usage":{"input_tokens":4,"output_tokens":2}}`))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// TestMessagesHistoryOwnership 验证随机引用绑定身份/别名/会话/真实连接，且普通偏好不能重绑定。
// 前置独立内存State；原引用可恢复、跨范围/混合连接/过期拒绝，引用不泄露原签名；内存随测试清理。
func TestMessagesHistoryOwnership(t *testing.T) {
	s := New(nil)
	p := &auth.Principal{UserID: "alice"}
	req := httptest.NewRequest("POST", "/v1/responses", nil)
	req.Header.Set("X-Session-Id", "session-a")
	first := s.PlanRoute(req, "model", map[string]any{"input": "Hello"}, p)
	target := dpcontract.RouteTarget{DeploymentID: "dep-a", ConnectionID: "connection-a", Model: "model", Pool: "model"}
	encoded, err := first.PrepareMessages(target, signedHistoryFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	output := encoded.Native["output"].([]any)
	ref := output[0].(map[string]any)["encrypted_content"].(string)
	if strings.Contains(ref, "test-signature") || len(ref) != len(llmdialogue.MessagesHistoryPrefix)+64 {
		t.Fatal("引用泄露签名或随机熵不足")
	}
	body := map[string]any{"input": output}
	next := s.PlanRoute(req, "model", body, p)
	if next.Err != nil || !next.Required || next.Target.ConnectionID != "connection-a" || len(next.MessagesHistory) != 1 {
		t.Fatalf("归属未恢复: %+v", next)
	}
	for _, tc := range []struct{ name, user, key, alias, session string }{
		{"user", "bob", "", "model", "session-a"}, {"key", "alice", "other-key", "model", "session-a"}, {"alias", "alice", "", "other", "session-a"}, {"session", "alice", "", "model", "session-b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := req.Clone(context.Background())
			r.Header.Set("X-Session-Id", tc.session)
			got := s.PlanRoute(r, tc.alias, body, &auth.Principal{UserID: tc.user, Hash: tc.key})
			if got.Err == nil {
				t.Fatal("跨范围引用被接受")
			}
		})
	}
	// 独立引用保存原连接，普通成功产生的新偏好不能覆盖它。
	first.Target = target
	first.Target.ConnectionID = "connection-b"
	s.CommitRoute(first, "dep-a", "other-result")
	if got := s.PlanRoute(req, "model", body, p); got.Err != nil || got.Target.ConnectionID != "connection-a" {
		t.Fatal("旧引用被普通请求重绑定")
	}
	other, err := first.PrepareMessages(first.Target, signedHistoryFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	mixed := append(append([]any(nil), output...), other.Native["output"].([]any)...)
	if s.PlanRoute(req, "model", map[string]any{"input": mixed}, p).Err == nil {
		t.Fatal("不同连接私有历史被混合")
	}
	s.affinitySetFor(messagesHistoryKey(first, ref), "{}", -time.Second)
	if s.PlanRoute(req, "model", body, p).Err == nil {
		t.Fatal("过期引用被接受")
	}
}

// brokenHistoryRedis 模拟共享存储故障；所有调用明确报错，不能使用本机旧副本冒充保存成功。
// 仅供单元测试，不持有外部资源。
type brokenHistoryRedis struct{}

// ReadAffinity 返回可识别存储失败；参数兼容缓存接口，无状态或外部副作用。
func (brokenHistoryRedis) ReadAffinity(context.Context, string) (string, error) {
	return "", errors.New("offline")
}

// CompareAffinity 返回写入失败；测试严格归属不能降级，忽略所有输入且不保存。
func (brokenHistoryRedis) CompareAffinity(context.Context, string, string, string, time.Duration) (string, error) {
	return "", errors.New("offline")
}

// WriteAffinity 返回确认失败；用于验证发布引用的失败路径，无数据写入。
func (brokenHistoryRedis) WriteAffinity(context.Context, string, string, time.Duration) error {
	return errors.New("offline")
}

// GetString 满足旧缓存接口，故障时没有可用值，无副作用。
func (brokenHistoryRedis) GetString(context.Context, string) (string, bool) { return "", false }

// SetString 满足旧缓存接口，测试故障不接受任何写入，无副作用。
func (brokenHistoryRedis) SetString(context.Context, string, string, time.Duration) {}

// TestMessagesHistoryStoreFailureAndMalformed 验证存储不可用、伪造版本与错误类型失败关闭。
// 前置独立故障缓存/内存State；发布失败无可用ref、恢复禁止本机降级；测试结束释放内存。
func TestMessagesHistoryStoreFailureAndMalformed(t *testing.T) {
	s := New(nil)
	req := httptest.NewRequest("POST", "/v1/responses", nil)
	req.Header.Set("X-Session-Id", "session")
	p := &auth.Principal{UserID: "alice"}
	plan := s.PlanRoute(req, "model", map[string]any{"input": "Hello"}, p)
	s.redis = brokenHistoryRedis{}
	if _, err := plan.PrepareMessages(dpcontract.RouteTarget{DeploymentID: "dep", ConnectionID: "connection"}, signedHistoryFixture(t)); err == nil {
		t.Fatal("缓存写失败却发布引用")
	}
	for _, ref := range []string{"xhub_messages_v2:" + strings.Repeat("a", 64), llmdialogue.MessagesHistoryPrefix + "bad", llmdialogue.MessagesHistoryPrefix + strings.Repeat("A", 64), llmdialogue.MessagesHistoryPrefix + strings.Repeat("a", 64)} {
		if got := s.PlanRoute(req, "model", map[string]any{"input": []any{map[string]any{"type": "reasoning", "encrypted_content": ref}}}, p); got.Err == nil {
			t.Fatalf("非法或未知引用被接受: %s", ref)
		}
	}
}
