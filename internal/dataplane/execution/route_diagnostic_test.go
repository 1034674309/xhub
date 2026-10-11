package execution

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sunqirui1987/xhub/internal/auth"
	"github.com/sunqirui1987/xhub/internal/config"
	"github.com/sunqirui1987/xhub/internal/router"
)

// diagnosticHost 用独立设置与运行状态驱动真实 Serve，复用无持久化的单元测试宿主。
type diagnosticHost struct {
	*logHost
	settings router.RouteSettings
	state    router.State
}

// RouteSettingsFor 返回测试的请求规则快照；参数身份由 Serve 提供，返回设置，不写数据。
func (h *diagnosticHost) RouteSettingsFor(*auth.Principal) router.RouteSettings { return h.settings }

// RouteState 返回测试冷却状态；Serve 调用，无参数，返回只读快照，无外部副作用。
func (h *diagnosticHost) RouteState() router.State { return h.state }

// TestServeRouteDiagnosisLogs 验证用户复现的零权重/过期 ID 场景，接口返回503并关联安全日志。
// 前置独立配置和内存日志；同时覆盖不兼容请求/无效协议失败，恢复日志输出，无真实凭据及上游调用。
func TestServeRouteDiagnosisLogs(t *testing.T) {
	for _, mode := range []string{"zero", "invalid", "capability", "fallback"} {
		t.Run(mode, func(t *testing.T) {
			dep := diagnosticDeployment("gpt-5.6-sol", "live", "bypass_openai_chat", false)
			s := router.BuiltinSettings()
			wantStatus, wantReason := 503, "deployment has no allocated traffic"
			if mode == "zero" {
				s.ModelDefaults = map[string]any{dep.ModelName: map[string]any{"allocations": []any{map[string]any{"deployment_id": "live", "weight": 0}, map[string]any{"deployment_id": "deleted", "weight": 100}}}}
			} else if mode == "invalid" {
				dep.ModelInfo["transport"] = "missing-transport"
				wantStatus, wantReason = 400, "model_info.transport must select a registered transport"
			} else if mode == "capability" {
				dep.ModelInfo["transport"] = "bypass_anthropic_messages"
				wantStatus, wantReason = 400, "strict"
			} else {
				s.ModelFallbacks = map[string]router.FallbackPolicy{dep.ModelName: {Fallbacks: []string{"fallback"}}}
				dep.ModelName = "fallback"
				dep.ModelInfo["transport"] = "missing-transport"
				wantStatus, wantReason = 400, "fallback_visited=1"
			}
			h := &diagnosticHost{logHost: newLogHost(&config.Config{ModelList: []config.ModelEntry{dep}}, nil), settings: s}
			buf := captureLog(t)
			req := chatRequest(t, "gpt-5.6-sol", false)
			if mode == "capability" {
				req = httptest.NewRequest(http.MethodPost, "/chat/completions", strings.NewReader(`{"model":"gpt-5.6-sol","messages":[{"role":"user","content":"hi"}],"tools":[{"type":"function","function":{"name":"f","strict":true,"parameters":{"type":"object"}}}]}`))
			}
			w := httptest.NewRecorder()
			Serve(h, w, req, "chat")
			if w.Code != wantStatus {
				t.Fatalf("%s 分类错误: %d %s", mode, w.Code, w.Body.String())
			}
			for _, fragment := range []string{"step=route_diagnosis", "call_id=\"" + w.Header().Get("x-litellm-call-id") + "\"", wantReason, "source=\"builtin\""} {
				if !strings.Contains(buf.String(), fragment) {
					t.Fatalf("缺失路由解释 %q: %s", fragment, buf.String())
				}
			}
			for _, secret := range []string{"arbitrary-private-secret", "private-password", "private-query", "https://"} {
				if strings.Contains(buf.String(), secret) {
					t.Fatalf("诊断泄漏敏感字段 %q", secret)
				}
			}
		})
	}
}

// diagnosticDeployment 构造执行链的诊断输入；参数为名称、部署、协议和暂停状态，返回内存配置，无外部数据。
func diagnosticDeployment(name, id, transport string, disabled bool) config.ModelEntry {
	return config.ModelEntry{ModelName: name, LiteLLMParams: map[string]any{
		"model": "real", "api_key": "arbitrary-private-secret", "api_base": "https://user:private-password@upstream.example/v1?token=private-query",
		"custom_llm_provider": "custom",
	}, ModelInfo: map[string]any{"id": id, "transport": transport, "disabled": disabled}}
}
