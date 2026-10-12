// 本文件验证 execution 的相关执行契约，阅读顺序为测试宿主/辅助方法，再到各个 Test 场景。
// 具体前置、期望、边界和清理方式见每个测试方法；夹具的空实现仅用于隔离当前测试。
package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/sunqirui1987/xhub/internal/config"
	providerregistry "github.com/sunqirui1987/xhub/internal/provider/registry"
	"github.com/sunqirui1987/xhub/internal/router"
)

// TestFallbackTransportFailures 验证连接错误和超时在输出前能回退，网关开关不会泄漏上游。
// 前置无网络的确定性HTTP传输；分别执行适配和原生入口，断言仅a/b两次及成功响应，无资源需清理。
// 参数 t 为 Go 测试上下文；返回值无，断言失败时终止当前测试。
func TestFallbackTransportFailures(t *testing.T) {
	for _, failure := range []struct {
		name string
		err  error
	}{{"connection", errors.New("connection reset")}, {"timeout", context.DeadlineExceeded}} {
		for _, native := range []bool{false, true} {
			t.Run(failure.name+map[bool]string{false: "-adapted", true: "-native"}[native], func(t *testing.T) {
				calls := []string{}
				client := &http.Client{Transport: auditRoundTripper(func(r *http.Request) (*http.Response, error) {
					var doc map[string]any
					if err := json.NewDecoder(r.Body).Decode(&doc); err != nil {
						t.Fatal(err)
					}
					model, _ := doc["model"].(string)
					calls = append(calls, model)
					if _, exists := doc["disable_fallbacks"]; exists {
						t.Fatal("网关字段发送至上游")
					}
					if model == "a" {
						return nil, failure.err
					}
					raw := []byte(`{"id":"ok","choices":[{"index":0,"message":{"role":"assistant","content":"recovered"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`)
					return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(bytes.NewReader(raw))}, nil
				})}
				models := []config.ModelEntry{Deployment("a", "a", "local", "https://example.invalid", "bypass_openai_chat", nil), Deployment("b", "b", "local", "https://example.invalid", "bypass_openai_chat", nil)}
				settings := router.BuiltinSettings()
				settings.ModelFallbacks = map[string]router.FallbackPolicy{"a": {Fallbacks: []string{"b"}}}
				// 适配宿主的chatConfig会规范化入口；原生场景保留部署声明的bypass入口。
				cfg := &config.Config{ModelList: models}
				if !native {
					cfg = chatConfig(models...)
				}
				w := httptest.NewRecorder()
				if native {
					h := &fallbackNativeHost{logicHost: &logicHost{cfg: cfg, client: client, models: models, pins: map[string]string{}}, policy: settings}
					r := httptest.NewRequest("POST", "/bypass/openai/v1/chat/completions", bytes.NewBufferString(`{"model":"a","messages":[{"role":"user","content":"hi"}],"disable_fallbacks":false}`))
					hit, ok := providerregistry.Match("POST", r.URL.Path)
					if !ok {
						t.Fatal("原生入口未匹配")
					}
					ServeBypass(h, w, r, hit)
				} else {
					h := &fallbackServeHost{logHost: newLogHost(cfg, client), settings: settings}
					Serve(h, w, chatRequest(t, "a", false), "chat")
				}
				if w.Code != 200 || !reflect.DeepEqual(calls, []string{"a", "b"}) {
					t.Fatalf("传输失败恢复错误: status=%d calls=%v body=%s", w.Code, calls, w.Body.String())
				}
			})
		}
	}
}
