package regression

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestMalformedNativeTerminalCannotPublishOwnership 验证原生 SSE 的损坏终态不能发布续接归属或产生费用。
// 参数 t 为上下文；真实路由、鉴权和数据库连接本地 Responses 上游，保留转发字节但日志必须失败。
// 请求结束后续接返回 400；本地服务与私有数据库 schema 由 cleanup 自动清理，不用外部凭据。
func TestMalformedNativeTerminalCannotPublishOwnership(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Errorf("原生路由错误: %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_broken\"}}\n\nevent: response.completed\ndata: {broken\n\n")
	}))
	t.Cleanup(upstream.Close)
	dep := chatDeployment("malformed-native")
	dep.ModelInfo = map[string]any{"id": "malformed-native-deployment", "transport": "bypass_openai_responses", "endpoint_types": []string{"responses", "bypass:openai-responses"}}
	dep.LiteLLMParams["api_base"] = upstream.URL + "/v1"
	h := newHarness(t, dep)
	admin := h.personalBillingSession()
	result := h.ok("POST", "/bypass/openai/v1/responses", admin, map[string]any{"model": dep.ModelName, "input": "first", "stream": true})
	if !strings.Contains(result.text(), "data: {broken") {
		t.Fatalf("原生流字节被修改: %s", result.describe())
	}
	detail := logDetail(t, h, admin, result.header("x-litellm-call-id"))
	if firstString(detail, "status") != "error" || firstFloat(detail, "spend") != 0 || !strings.Contains(firstString(detail, "error"), "without successful completion") {
		t.Fatalf("损坏终态被记为成功或计费: %v", detail)
	}
	next := h.do("POST", "/responses", admin, map[string]any{"model": dep.ModelName, "input": "next", "previous_response_id": "resp_broken"})
	if next.status != 400 || !strings.Contains(next.text(), "response ownership is unknown") {
		t.Fatalf("损坏终态发布了归属: %s", next.describe())
	}
}
