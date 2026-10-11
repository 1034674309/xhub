package execution

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sunqirui1987/xhub/internal/auth"
	"github.com/sunqirui1987/xhub/internal/config"
	dpcontract "github.com/sunqirui1987/xhub/internal/dataplane/contract"
	"github.com/sunqirui1987/xhub/internal/gateway/prefs"
	providerregistry "github.com/sunqirui1987/xhub/internal/provider/registry"
	"github.com/sunqirui1987/xhub/internal/router"
)

// fallbackNativeHost 为原生执行测试提供模型策略；内嵌宿主记录转发与计费，无共享配置修改。
type fallbackNativeHost struct {
	*logicHost
	policy prefs.RouteSettings
}

// RouteSettingsFor 返回原生执行测试指定的配置；参数为鉴权主体，调用方ServeBypass，无持久化副作用。
func (h *fallbackNativeHost) RouteSettingsFor(*auth.Principal) prefs.RouteSettings { return h.policy }

// TestNativeDedicatedFallbackChain 验证仅配置专用列表时，第一目标5xx后继续剩余目标或目标自身的通用链。
// 参数 t 为测试上下文，无返回；本地HTTP模拟上下文和内容错误，验证普通4xx终止和异步禁用，Cleanup关闭服务。
func TestNativeDedicatedFallbackChain(t *testing.T) {
	for _, tc := range []struct {
		name, code         string
		child, background  bool
		middleStatus, want int
		models             string
	}{
		{"context-pending", "context_length_exceeded", false, false, 500, 200, "a,b,c"},
		{"content-child", "content_policy_violation", true, false, 500, 200, "a,b,c"},
		{"ordinary-terminal", "context_length_exceeded", false, false, 400, 400, "a,b"},
		{"background-disabled", "context_length_exceeded", false, true, 500, 400, "a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := []string{}
			up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var doc map[string]any
				json.NewDecoder(r.Body).Decode(&doc)
				model, _ := doc["model"].(string)
				calls = append(calls, model)
				w.Header().Set("Content-Type", "application/json")
				switch model {
				case "a":
					w.WriteHeader(400)
					fmt.Fprintf(w, "{\"error\":{\"code\":%q}}", tc.code)
				case "b":
					w.WriteHeader(tc.middleStatus)
					io.WriteString(w, "{\"error\":{\"code\":\"api_error\"}}")
				default:
					io.WriteString(w, "{\"id\":\"ok\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}")
				}
			}))
			t.Cleanup(up.Close)
			settings := prefs.BuiltinSettings()
			targets := []string{"b", "c"}
			if tc.child {
				targets = targets[:1]
			}
			p := router.FallbackPolicy{ContextWindow: targets}
			if tc.code == "content_policy_violation" {
				p = router.FallbackPolicy{ContentPolicy: targets}
			}
			settings.ModelFallbacks = map[string]router.FallbackPolicy{"a": p, "b": {Fallbacks: []string{"c"}}}
			h := &fallbackNativeHost{logicHost: officialHost(up, Deployment("a", "a", "key", up.URL, "bypass_openai_responses", nil), Deployment("b", "b", "key", up.URL, "bypass_openai_responses", nil), Deployment("c", "c", "key", up.URL, "bypass_openai_responses", nil)), policy: settings}
			req := httptest.NewRequest("POST", "/bypass/openai/v1/responses", strings.NewReader(fmt.Sprintf("{\"model\":\"a\",\"input\":\"hi\",\"background\":%t}", tc.background)))
			hit, ok := providerregistry.Match("POST", req.URL.Path)
			if !ok {
				t.Fatal("原生入口未匹配")
			}
			w := httptest.NewRecorder()
			ServeBypass(h, w, req, hit)
			if w.Code != tc.want || strings.Join(calls, ",") != tc.models {
				t.Fatalf("状态%d 调用%v 正文%s", w.Code, calls, w.Body.String())
			}
		})
	}
}

// fallbackServeHost 为执行单测提供请求私有策略和可观察结果；内存配置不修改共享宿主。
type fallbackServeHost struct {
	*logHost
	settings prefs.RouteSettings
	deny     string
	required bool
}

// RouteSettingsFor 返回单测指定策略；参数为已鉴权主体，无存储或副作用。
func (h *fallbackServeHost) RouteSettingsFor(*auth.Principal) prefs.RouteSettings { return h.settings }

// WriteChatJSON 记录真实HTTP响应；参数与执行宿主一致，返回无，写入测试响应而不结算。
func (h *fallbackServeHost) WriteChatJSON(w http.ResponseWriter, _ *auth.Principal, _, _, _, _, _ string, body []byte, status int, _ time.Time, _ string) {
	w.WriteHeader(status)
	w.Write(body)
}

// EnforceIdentityLimits 模拟单个目标被拒绝；参数为目标模型，返回许可并在拒绝时写403，无外部权限数据。
func (h *fallbackServeHost) EnforceIdentityLimits(w http.ResponseWriter, _ string, _ *auth.Principal, model string, _ int) bool {
	if model == h.deny {
		w.WriteHeader(403)
		return false
	}
	return true
}

// PlanRoute 返回单测固定部署约束；参数为请求及身份，返回固定计划，无外部会话写入。
func (h *fallbackServeHost) PlanRoute(*http.Request, string, map[string]any, *auth.Principal) dpcontract.RoutePlan {
	if h.required {
		return dpcontract.RoutePlan{Required: true, Pinned: router.CooldownID(h.cfg.ModelList[0])}
	}
	return dpcontract.RoutePlan{}
}

// TestServeModelFallback 验证实际HTTP执行顺序、禁用开关、继续请求和权限拒绝。
// 前置本地上游和内存路由，所有错误与成功均走真实HTTP，服务由Cleanup关闭。
func TestServeModelFallback(t *testing.T) {
	for _, tc := range []struct {
		name, denied      string
		disable, required bool
		want              int
		calls             int
	}{{"enabled", "", false, false, 200, 2}, {"disabled", "", true, false, 502, 1}, {"continuation", "", false, true, 502, 1}, {"denied", "b", false, false, 403, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			calls := []string{}
			up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var b map[string]any
				json.NewDecoder(r.Body).Decode(&b)
				model, _ := b["model"].(string)
				calls = append(calls, model)
				if b["disable_fallbacks"] != nil {
					t.Error("开关发送给了上游")
				}
				if model == "a" {
					w.WriteHeader(500)
					io.WriteString(w, "{}")
					return
				}
				io.WriteString(w, "{\"id\":\"ok\",\"choices\":[{\"message\":{\"role\":\"assistant\",\"content\":\"ok\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1}}")
			}))
			t.Cleanup(up.Close)
			cfg := chatConfig(config.ModelEntry{ModelName: "a", LiteLLMParams: map[string]any{"model": "a", "api_base": up.URL, "api_key": "sk-local", "deployment_id": "a"}}, config.ModelEntry{ModelName: "b", LiteLLMParams: map[string]any{"model": "b", "api_base": up.URL, "api_key": "sk-local", "deployment_id": "b"}})
			settings := prefs.BuiltinSettings()
			settings.ModelFallbacks = map[string]router.FallbackPolicy{"a": {Fallbacks: []string{"b"}}}
			h := &fallbackServeHost{logHost: newLogHost(cfg, up.Client()), settings: settings, deny: tc.denied, required: tc.required}
			req := chatRequest(t, "a", false)
			if tc.disable {
				req.Body = io.NopCloser(strings.NewReader("{\"model\":\"a\",\"messages\":[{\"role\":\"user\",\"content\":\"hi\"}],\"disable_fallbacks\":true}"))
			}
			w := httptest.NewRecorder()
			Serve(h, w, req, "chat")
			if w.Code != tc.want || len(calls) != tc.calls {
				t.Fatalf("%s: 状态%d 调用%v 正文%s", tc.name, w.Code, calls, w.Body.String())
			}
		})
	}
}

// TestFallbackStreamAlreadyWritten 验证配置回退后流式中断仍不切换；前置分段错误流，断言只发送一次。
// 使用内存RoundTripper，响应和缓存随测试结束释放，不依赖外部供应商。
func TestFallbackStreamAlreadyWritten(t *testing.T) {
	attempts := 0
	client := &http.Client{Transport: auditRoundTripper(func(*http.Request) (*http.Response, error) {
		attempts++
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(&FragmentedErrorReader{Parts: [][]byte{[]byte("data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n")}, Err: errors.New("reset")})}, nil
	})}
	cfg := chatConfig(config.ModelEntry{ModelName: "a", LiteLLMParams: map[string]any{"model": "a", "api_base": "https://example.invalid", "api_key": "local"}}, config.ModelEntry{ModelName: "b", LiteLLMParams: map[string]any{"model": "b", "api_base": "https://example.invalid", "api_key": "local"}})
	settings := prefs.BuiltinSettings()
	settings.ModelFallbacks = map[string]router.FallbackPolicy{"a": {Fallbacks: []string{"b"}}}
	h := &fallbackServeHost{logHost: newLogHost(cfg, client), settings: settings}
	w := httptest.NewRecorder()
	Serve(h, w, chatRequest(t, "a", true), "chat")
	if attempts != 1 || !strings.Contains(w.Body.String(), "partial") {
		t.Fatalf("输出后重试: 次数%d 正文%s", attempts, w.Body.String())
	}
}

// TestNativeGroupMemberFallback 验证组名原生调用使用实际成员的回退链；参数t为测试上下文，无返回。
// 前置本地上游及单成员组；覆盖成功回退、显式禁用、异步禁用和目标错误，Cleanup关闭HTTP服务。
func TestNativeGroupMemberFallback(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		disabled, background bool
		targetStatus, want   int
		calls                string
	}{
		{"member-chain", false, false, 200, 200, "a,b"},
		{"disabled", true, false, 200, 500, "a"},
		{"background", false, true, 200, 500, "a"},
		{"target-error", false, false, 400, 400, "a,b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := []string{}
			up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				json.NewDecoder(r.Body).Decode(&body)
				model, _ := body["model"].(string)
				calls = append(calls, model)
				w.Header().Set("Content-Type", "application/json")
				status := tc.targetStatus
				if model == "a" {
					status = 500
				}
				w.WriteHeader(status)
				if status >= 400 {
					io.WriteString(w, "{\"error\":{\"code\":\"api_error\"}}")
				} else {
					io.WriteString(w, "{\"id\":\"ok\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}")
				}
			}))
			t.Cleanup(up.Close)
			settings := prefs.BuiltinSettings()
			settings.RoutingGroups = []router.Group{{Name: "group", Models: []string{"a"}, Strategy: "simple-shuffle"}}
			settings.ModelFallbacks = map[string]router.FallbackPolicy{"a": {Fallbacks: []string{"b"}}}
			h := &fallbackNativeHost{logicHost: officialHost(up,
				Deployment("a", "a", "key", up.URL, "bypass_openai_responses", nil),
				Deployment("b", "b", "key", up.URL, "bypass_openai_responses", nil)), policy: settings}
			req := httptest.NewRequest("POST", "/bypass/openai/v1/responses", strings.NewReader(fmt.Sprintf("{\"model\":\"group\",\"input\":\"hi\",\"background\":%t,\"disable_fallbacks\":%t}", tc.background, tc.disabled)))
			hit, ok := providerregistry.Match("POST", req.URL.Path)
			if !ok {
				t.Fatal("原生入口未匹配")
			}
			w := httptest.NewRecorder()
			ServeBypass(h, w, req, hit)
			if w.Code != tc.want || strings.Join(calls, ",") != tc.calls {
				t.Fatalf("组成员回退: 状态%d 调用%v 正文%s", w.Code, calls, w.Body.String())
			}
		})
	}
}
