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

type FragmentedErrorReader struct {
	Parts [][]byte
	Err   error
}

// Read 分段返回测试流；参数为读缓冲，返回字节数或预设错误。仅内存测试，无清理副作用。
func (r *FragmentedErrorReader) Read(p []byte) (int, error) {
	if len(r.Parts) == 0 {
		return 0, r.Err
	}
	part := r.Parts[0]
	r.Parts = r.Parts[1:]
	return copy(p, part), nil
}
