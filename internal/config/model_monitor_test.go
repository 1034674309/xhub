package config

import "testing"

// TestMonitorIdentity 使用内存部署验证命名连接、实际模型回退和敏感地址剥离；无外部数据需要清理。
func TestMonitorIdentity(t *testing.T) {
	for _, tc := range []struct {
		name            string
		params          map[string]any
		supplier, model string
	}{
		{"named", map[string]any{"litellm_credential_name": "supplier-a", "model": "actual"}, "supplier-a", "actual"},
		{"inline sanitized", map[string]any{"api_base": "https://user:secret@relay.example/v1?key=secret#private", "model": "actual"}, "https://relay.example/v1", "actual"},
		{"escaped path", map[string]any{"api_base": "https://relay.example/a%20b"}, "https://relay.example/a%20b", "alias"},
		{"invalid url", map[string]any{"api_base": "http://%zz"}, "", "alias"},
		{"wrong type", map[string]any{"model": 123, "api_base": 123}, "", "alias"},
		{"missing", nil, "", "alias"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			supplier, model := (ModelEntry{ModelName: "alias", LiteLLMParams: tc.params}).MonitorIdentity()
			if supplier != tc.supplier || model != tc.model {
				t.Fatalf("监控身份错误: %q/%q，期望 %q/%q", supplier, model, tc.supplier, tc.model)
			}
		})
	}
}

// TestLogAPIBase 验证正常、缺失、非法及携带秘密的地址；纯内存输入，无持久数据需要清理。
// 参数 t 为测试上下文；返回无；确保日志地址仅保留 HTTP 协议、主机与转义路径。
func TestLogAPIBase(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{"https://relay.example/v1", "https://relay.example/v1"},
		{"https://user:secret@relay.example/a%20b?key=secret#private", "https://relay.example/a%20b"},
		{"", ""}, {"http://%zz", ""}, {"/relative", ""}, {"ftp://relay.example", ""},
	} {
		got := (ModelEntry{LiteLLMParams: map[string]any{"api_base": tc.raw}}).LogAPIBase()
		if got != tc.want {
			t.Fatalf("上游基础地址脱敏错误：%q，期望 %q", got, tc.want)
		}
	}
}
