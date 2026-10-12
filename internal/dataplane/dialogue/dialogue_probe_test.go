// 本文件验证 dialogue 的相关执行契约，阅读顺序为测试宿主/辅助方法，再到各个 Test 场景。
// 具体前置、期望、边界和清理方式见每个测试方法；夹具的空实现仅用于隔离当前测试。
package dialogue

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/sunqirui1987/xhub/internal/config"
)

// probeRoundTripper 是连接探测测试的函数型 HTTP 传输适配器，所有结果由进程内回调提供，不访问供应商。
type probeRoundTripper func(*http.Request) (*http.Response, error)

// RoundTrip 执行测试回调；参数为待发请求，返回测试指定响应和错误；仅供 ProbeDialogue 单元测试。
func (f probeRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// probeDeployment 构造显式 OpenAI Chat 传输部署；参数为上游地址，返回带假凭据的服务端部署，无持久化副作用。
// 供本包测试调用，不创建外部数据；资源由所属测试管理。
func probeDeployment(base string) config.ModelEntry {
	return config.ModelEntry{ModelName: "probe", LiteLLMParams: map[string]any{
		"model": "upstream-probe", "api_base": base, "api_key": "secret-probe",
	}, ModelInfo: map[string]any{"transport": "bypass_openai_chat", "endpoint_types": []string{"chat"}}}
}

// TestProbeDialogueContracts 验证探测会发送最小英文请求并区分成功、上游状态和损坏协议；全部使用内存客户端且无需清理外部数据。
// 参数 t 为 Go 测试上下文；返回值无，断言失败时终止当前测试。
func TestProbeDialogueContracts(t *testing.T) {
	tests := []struct {
		name, body string
		status     int
		wantErr    string
	}{
		{name: "valid", status: 200, body: `{"id":"ok","choices":[{"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`},
		{name: "unauthorized", status: 401, body: `{"error":"bad key"}`, wantErr: "HTTP 401"},
		{name: "invalid protocol", status: 200, body: `{}`, wantErr: "configured protocol"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &http.Client{Transport: probeRoundTripper(func(r *http.Request) (*http.Response, error) {
				raw, _ := io.ReadAll(r.Body)
				if !strings.Contains(string(raw), "Reply with OK.") || !strings.HasSuffix(r.URL.Path, "/chat/completions") {
					t.Fatalf("unexpected probe request %s %s", r.URL.Path, raw)
				}
				if got := r.Header.Get("Authorization"); got != "Bearer secret-probe" {
					t.Fatalf("authorization = %q", got)
				}
				return &http.Response{StatusCode: tt.status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(tt.body))}, nil
			})}
			err := ProbeDialogue(t.Context(), client, probeDeployment("https://probe.invalid/v1"))
			if tt.wantErr == "" && err != nil {
				t.Fatalf("ProbeDialogue() error = %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("ProbeDialogue() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

// TestProbeDialogueRedactsNetworkFailure 验证网络错误不会回显带密钥的目标或底层错误；使用内存失败客户端，无外部数据需清理。
// 参数 t 为 Go 测试上下文；返回值无，断言失败时终止当前测试。
func TestProbeDialogueRedactsNetworkFailure(t *testing.T) {
	client := &http.Client{Transport: probeRoundTripper(func(*http.Request) (*http.Response, error) {
		return nil, io.ErrUnexpectedEOF
	})}
	err := ProbeDialogue(t.Context(), client, probeDeployment("https://secret.invalid/v1?api_key=hidden"))
	if err == nil || err.Error() != "upstream request failed" {
		t.Fatalf("ProbeDialogue() error = %v", err)
	}
}

// failingProbeReader 首次提供一个 JSON 起始字节，随后报告带敏感哨兵的读取错误，验证探测只暴露安全分类。
type failingProbeReader struct {
	// 是否已返回首次 JSON 起始字节，后续读取转入固定失败。
	sent bool
}

// Read 首次返回片段、随后返回固定错误；参数是目标缓冲区，返回字节数和测试错误。
func (r *failingProbeReader) Read(p []byte) (int, error) {
	if r.sent {
		return 0, errors.New("reader secret https://secret.invalid?key=hidden")
	}
	r.sent = true
	return copy(p, []byte("{")), nil
}

// TestProbeDialogueBoundaries 验证 nil 客户端、不支持协议、超限正文和读取失败均返回脱敏分类错误；不访问外部服务。
// 参数 t 为 Go 测试上下文；返回值无，断言失败时终止当前测试。
// 测试只操作内存配置或本地假上游，HTTP 服务及正文由本例关闭，不创建外部付费任务或持久化数据。
func TestProbeDialogueBoundaries(t *testing.T) {
	tests := []struct {
		name string
		dep  config.ModelEntry
		cli  *http.Client
		kind DialogueProbeErrorKind
	}{
		{name: "nil client", dep: probeDeployment("https://probe.invalid/v1"), kind: DialogueProbeUnavailable},
		{name: "unsupported", dep: config.ModelEntry{ModelInfo: map[string]any{"transport": "openai_image_generation", "endpoint_types": []string{"image_generation"}}}, cli: &http.Client{}, kind: DialogueProbeUnsupported},
		{name: "oversize", dep: probeDeployment("https://probe.invalid/v1"), cli: &http.Client{Transport: probeRoundTripper(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", (1<<20)+1)))}, nil
		})}, kind: DialogueProbeOversize},
		{name: "read error", dep: probeDeployment("https://probe.invalid/v1"), cli: &http.Client{Transport: probeRoundTripper(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(&failingProbeReader{})}, nil
		})}, kind: DialogueProbeUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ProbeDialogue(t.Context(), tt.cli, tt.dep)
			var probeErr *DialogueProbeError
			if !errors.As(err, &probeErr) || probeErr.Kind != tt.kind {
				t.Fatalf("ProbeDialogue() error = %#v, want kind %q", err, tt.kind)
			}
			if strings.Contains(err.Error(), "secret.invalid") || strings.Contains(err.Error(), "hidden") {
				t.Fatalf("探测错误泄露底层信息: %v", err)
			}
		})
	}
}
