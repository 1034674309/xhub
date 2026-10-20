package regression

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

// TestMessagesCitationsContinuation 验证真实 Messages 路由接受客户端空引用历史，保留原生真实引用并拒绝非法结构。
// 前置真实 PostgreSQL、有限额 Key、本机上游；检查同步/流式终态、出站原文、费用和零外发错误。
// 所有输入为合成数据，harness 和 httptest 在结束后回收 schema、身份、Key 及服务，不使用外部凭据。
func TestMessagesCitationsContinuation(t *testing.T) {
	t.Setenv("XHUB_REGRESSION_REDIS_URL", "")
	var calls atomic.Int32
	var expected atomic.Value
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if !reflect.DeepEqual(body["messages"], expected.Load()) {
			t.Error("原生历史引用被删除或改变")
		}
		w.Header().Set("Content-Type", "application/json")
		if body["stream"] == true {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, streamTerminalFrames("messages"))
			return
		}
		_, _ = io.WriteString(w, `{"id":"msg_citations","type":"message","role":"assistant","content":[{"type":"text","text":"TERMINAL_OK"}],"stop_reason":"end_turn","usage":{"input_tokens":8,"output_tokens":2}}`)
	}))
	t.Cleanup(up.Close)
	dep := hardBudgetDeployment("claude-citations")
	dep.LiteLLMParams["api_base"] = up.URL
	dep.LiteLLMParams["custom_llm_provider"] = "anthropic"
	dep.ModelInfo["transport"] = "bypass_anthropic_messages"
	dep.ModelInfo["endpoint_types"] = []string{"messages"}
	h := newHarness(t, dep)
	admin := h.adminSession()
	owner := h.openScope(t, admin, dep.ModelName)
	h.setKeyBudget(t, admin, owner.key, 1)
	success := 0
	for _, tc := range []struct {
		name, field string
		invalid     bool
	}{
		{"missing", "", false}, {"null", `,"citations":null`, false}, {"empty", `,"citations":[]`, false},
		{"real", `,"citations":[{"type":"char_location","cited_text":"Example","document_index":0,"start_char_index":0,"end_char_index":7}]`, false},
		{"invalid", `,"citations":"bad"`, true}, {"invalid_item", `,"citations":[null]`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, stream := range []bool{false, true} {
				var body map[string]any
				_ = json.Unmarshal([]byte(`{"max_tokens":64,"messages":[{"role":"user","content":"Hello."},{"role":"assistant","content":[{"type":"text","text":"(no content)"`+tc.field+`}]},{"role":"user","content":"Continue."}]}`), &body)
				body["model"], body["stream"] = dep.ModelName, stream
				expected.Store(body["messages"])
				before := calls.Load()
				got := h.do(http.MethodPost, "/v1/messages", owner.key, body)
				if tc.invalid {
					if got.status != 400 || calls.Load() != before || !strings.Contains(got.text(), "citation") {
						t.Fatalf("非法引用未零外发拒绝: %s", got.describe())
					}
					continue
				}
				if got.status != 200 || !strings.Contains(got.text(), "TERMINAL_OK") || calls.Load() != before+1 {
					t.Fatalf("引用续轮失败: %s", got.describe())
				}
				if stream && !strings.Contains(got.text(), "message_stop") {
					t.Fatal("流缺少结束事件")
				}
				success++
			}
		})
	}
	holds, err := h.db.PendingKeyBudget(t.Context(), owner.keyID)
	if err != nil || len(holds) != 0 {
		t.Fatalf("引用续轮留下预占: %v %v", holds, err)
	}
	key, err := h.db.GetKey(t.Context(), owner.keyID)
	if err != nil || !nearlyEqual(key.Spend, float64(success)*0.000032) {
		t.Fatalf("引用请求计费错误: success=%d key=%+v err=%v", success, key, err)
	}
}

// TestMessagesCitationsBridgeBoundary 验证真实跨协议路由只忽略无语义的空引用，非空引用在外发前明确拒绝。
// 前置隔离数据库与本地 Chat 上游；检查输出、零外发错误、费用和预占，无付费调用，harness回收全部数据。
func TestMessagesCitationsBridgeBoundary(t *testing.T) {
	t.Setenv("XHUB_REGRESSION_REDIS_URL", "")
	var calls atomic.Int32
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		raw, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(raw), "(no content)") || strings.Contains(string(raw), `"citations"`) {
			t.Error("桥接文本丢失或引用占位泄漏")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"chat_citations","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"BRIDGE_OK"},"finish_reason":"stop"}],"usage":{"prompt_tokens":8,"completion_tokens":2,"total_tokens":10}}`)
	}))
	t.Cleanup(up.Close)
	dep := hardBudgetDeployment("chat-citations")
	dep.LiteLLMParams["api_base"] = up.URL
	h := newHarness(t, dep)
	admin := h.adminSession()
	owner := h.openScope(t, admin, dep.ModelName)
	h.setKeyBudget(t, admin, owner.key, 1)
	for _, tc := range []struct {
		name, field string
		denied      bool
	}{
		{"missing", "", false}, {"null", `,"citations":null`, false}, {"empty", `,"citations":[]`, false},
		{"real", `,"citations":[{"type":"char_location","cited_text":"Example","document_index":0,"start_char_index":0,"end_char_index":7}]`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body map[string]any
			_ = json.Unmarshal([]byte(`{"max_tokens":64,"messages":[{"role":"assistant","content":[{"type":"text","text":"(no content)"`+tc.field+`}]},{"role":"user","content":"Continue."}]}`), &body)
			body["model"] = dep.ModelName
			before := calls.Load()
			got := h.do(http.MethodPost, "/v1/messages", owner.key, body)
			if tc.denied {
				if got.status != 400 || calls.Load() != before || !strings.Contains(got.text(), "citations") {
					t.Fatalf("非空引用被有损桥接: %s", got.describe())
				}
				return
			}
			if got.status != 200 || !strings.Contains(got.text(), "BRIDGE_OK") || calls.Load() != before+1 {
				t.Fatalf("空引用桥接失败: %s", got.describe())
			}
		})
	}
	holds, err := h.db.PendingKeyBudget(t.Context(), owner.keyID)
	if err != nil || len(holds) != 0 {
		t.Fatalf("桥接残留预占: %v %v", holds, err)
	}
	key, err := h.db.GetKey(t.Context(), owner.keyID)
	if err != nil || !nearlyEqual(key.Spend, 3*0.000032) {
		t.Fatalf("桥接计费错误: %+v %v", key, err)
	}
}
