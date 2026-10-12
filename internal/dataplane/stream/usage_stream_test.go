// 本文件验证 stream 的相关执行契约，阅读顺序为测试宿主/辅助方法，再到各个 Test 场景。
// 具体前置、期望、边界和清理方式见每个测试方法；夹具的空实现仅用于隔离当前测试。
package stream

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	dpusage "github.com/sunqirui1987/xhub/internal/dataplane/usage"
)

// TestStreamUsageMergesAnthropicFramesAndNestedProviders 验证分帧 Anthropic 用量、Responses 嵌套用量及 Gemini 思考/缓存统计被保留与合并。
// 参数 t 为 Go 测试上下文；返回值无，断言失败定位具体协议、路由或日志行为。
// 前置使用独立测试配置及内存传输/纯函数输入，不依赖外部供应商凭据。
// 测试状态只存在本例内存，不产生持久化数据。
func TestStreamUsageMergesAnthropicFramesAndNestedProviders(t *testing.T) {
	var usage map[string]any
	usage = streamUsage([]byte("data: {\"usage\":{\"input_tokens\":12,\"cache_read_input_tokens\":7}}\n"), usage)
	usage = streamUsage([]byte("data: {\"usage\":{\"output_tokens\":5}}\n"), usage)
	if pt, ct := dpusage.UsageCounts(usage); pt != 12 || ct != 5 || usage["cache_read_input_tokens"] != float64(7) {
		t.Fatalf("merged Anthropic usage: %#v", usage)
	}

	responses := streamUsage([]byte("data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":9,\"output_tokens\":4}}}\n"), nil)
	if pt, ct := dpusage.UsageCounts(responses); pt != 9 || ct != 4 {
		t.Fatalf("nested Responses usage: %#v", responses)
	}

	gemini := streamUsage([]byte("data: {\"usageMetadata\":{\"promptTokenCount\":20,\"candidatesTokenCount\":6,\"thoughtsTokenCount\":2,\"cachedContentTokenCount\":11}}\n"), nil)
	if pt, ct := dpusage.UsageCounts(gemini); pt != 20 || ct != 8 {
		t.Fatalf("Gemini usage: %#v", gemini)
	}
	details := gemini["prompt_tokens_details"].(map[string]any)
	if details["cached_tokens"] != 11 {
		t.Fatalf("Gemini cached input: %#v", gemini)
	}
}

// TestPipeStreamParsesFragmentedSSEAndReturnsBodyError 验证 SSE 在字段和 CRLF 中间分块仍可提取用量，转发保留字节且原读取错误返回调用方。
// 参数 t 为 Go 测试上下文；返回值无，断言失败定位具体协议、路由或日志行为。
// 前置使用独立测试配置及内存传输/纯函数输入，不依赖外部供应商凭据。
// 测试状态只存在本例内存，不产生持久化数据。
func TestPipeStreamParsesFragmentedSSEAndReturnsBodyError(t *testing.T) {
	readErr := errors.New("upstream reset")
	body := &FragmentedErrorReader{Parts: [][]byte{
		[]byte("da"),
		[]byte("ta: {\"usageMeta"),
		[]byte("data\":{\"promptTokenCount\":13,\"candidatesTokenCount\":2,\"cachedContentTokenCount\":8}}\r"),
		[]byte("\n\r\n"),
	}, Err: readErr}
	resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(body), Header: make(http.Header)}
	w := httptest.NewRecorder()
	wrote, usage, _, captured, gotErr := PipeStream(w, resp, time.Now())
	if !wrote || !errors.Is(gotErr, readErr) {
		t.Fatalf("wrote=%v err=%v", wrote, gotErr)
	}
	if pt, ct := dpusage.UsageCounts(usage); pt != 13 || ct != 2 {
		t.Fatalf("fragmented usage: %#v", usage)
	}
	if len(captured) == 0 || captured[len(captured)-1] != '\n' {
		t.Fatalf("captured stream: %q", captured)
	}
}

// FragmentedErrorReader 按 Parts 顺序返回片段，耗尽后返回 Err，模拟 SSE 在任意读取边界分块或断流。
// 每次 Read 弹出整个片段；缓冲区比片段小时未拷贝部分会丢失，因此测试应提供足够大的读取缓冲。
type FragmentedErrorReader struct {
	// 尚未返回的字节片段，Read 会弹出首项，测试需保证读取缓冲足够大。
	Parts [][]byte
	// 片段耗尽后返回的预设错误，通常为断流错误。
	Err error
}

// Read 为断流测试按顺序返回片段。
// 参数 p 为目标缓冲区；返回拷贝字节数与 nil，Parts 耗尽后返回 0 与 Err。
// 读取会弹出一个完整片段，p 太小时未拷贝尾部不保留；只适用于本测试的大缓冲读取，无外部资源。
func (r *FragmentedErrorReader) Read(p []byte) (int, error) {
	if len(r.Parts) == 0 {
		return 0, r.Err
	}
	part := r.Parts[0]
	r.Parts = r.Parts[1:]
	return copy(p, part), nil
}
