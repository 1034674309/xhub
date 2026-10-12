package regression

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// cacheObserverRoundTripper 将确定性 HTTP 回调用作测试底层传输，不访问网络或读取真实凭据。
type cacheObserverRoundTripper func(*http.Request) (*http.Response, error)

// RoundTrip 将 req 交给测试回调并原样返回结果，供观测器正常及失败路径验证使用。
func (f cacheObserverRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// TestSessionCacheObserverEvidence 验证读后恢复响应、准确保存缓存正数/零/缺失及网络失败。
// 前置为本地确定性 transport；子测试无需数据库、外部凭据或数据清理，失败定位到具体证据语义。
func TestSessionCacheObserverEvidence(t *testing.T) {
	for _, tc := range []struct {
		// name 标识可单独定位的业务边界。
		name string
		// body 是模拟供应商的原始正文，也是读后恢复的比较基准。
		body string
		// present 表示 cached_tokens 是否明确存在；缺失不能被补成零。
		present bool
		// cached 是字段明确存在时的期望值。
		cached float64
	}{
		{"positive", "{\"usage\":{\"prompt_tokens\":4096,\"prompt_tokens_details\":{\"cached_tokens\":3968}}}", true, 3968},
		{"explicit_zero", "{\"usage\":{\"prompt_tokens\":4096,\"prompt_tokens_details\":{\"cached_tokens\":0}}}", true, 0},
		{"missing", "{\"usage\":{\"prompt_tokens\":4096,\"prompt_tokens_details\":{}}}", false, 0},
		{"invalid_json", "upstream unavailable", false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// 本地回调仅返回指定正文，验证观察器不会改变网关随后读取的内容。
			o := &sessionCacheTransport{vendors: map[string]string{"supplier.test": "supplier"}, next: cacheObserverRoundTripper(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})}
			req, _ := http.NewRequest("POST", "https://supplier.test/v1/chat/completions", nil)
			resp, err := o.RoundTrip(req)
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if string(raw) != tc.body || len(o.attempts) != 1 || o.attempts[0].Vendor != "supplier" || o.attempts[0].Status != 200 {
				t.Fatal("观察器修改正文或漏记真实尝试")
			}
			details, _ := o.attempts[0].Usage["prompt_tokens_details"].(map[string]any)
			value, present := details["cached_tokens"]
			if present != tc.present || present && value != tc.cached {
				t.Fatalf("缓存证据语义错误: value=%v present=%v", value, present)
			}
		})
	}
	// 网络失败不能被当成收到正常 HTTP 响应但缓存未命中的样本。
	t.Run("network_error", func(t *testing.T) {
		want := errors.New("test connection failure")
		o := &sessionCacheTransport{next: cacheObserverRoundTripper(func(*http.Request) (*http.Response, error) { return nil, want })}
		req, _ := http.NewRequest("POST", "https://supplier.test/v1/chat/completions", nil)
		resp, err := o.RoundTrip(req)
		if resp != nil || !errors.Is(err, want) || len(o.attempts) != 1 || o.attempts[0].Status != 0 || o.attempts[0].Usage != nil {
			t.Fatal("连接失败被转换成正常缓存样本")
		}
	})
}
