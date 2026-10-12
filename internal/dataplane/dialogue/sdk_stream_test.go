// 本文件验证 dialogue 的相关执行契约，阅读顺序为测试宿主/辅助方法，再到各个 Test 场景。
// 具体前置、期望、边界和清理方式见每个测试方法；夹具的空实现仅用于隔离当前测试。
package dialogue

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	llmdialogue "github.com/sunqirui1987/xhub/internal/llm/dialogue"
)

// TestOfficialSSEFraming 前置注释、CRLF、多行 data 和残缺终态，验证官方分帧和失败传播。
// 参数 t 为测试上下文；全程使用内存响应，PipeDialogue 关闭响应体，不留下外部数据。
func TestOfficialSSEFraming(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		valid     bool
	}{
		{"comments_crlf_multiline", ": heartbeat\r\n\r\nevent: message\r\ndata: {\"id\":\"x\",\r\ndata: \"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\r\n\r\ndata: [DONE]\r\n\r\n", true},
		{"unterminated_completion", "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\ndata: [DONE]\n", false},
		{"malformed_json", "data: {bad}\n\n", false},
		{"upstream_error", "event: error\ndata: {\"error\":{\"message\":\"unavailable\"}}\n\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			resp := &http.Response{Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(tc.raw))}
			completed := false
			_, _, _, raw, err := PipeDialogue(w, resp, time.Now(), "public", "openai-chat", "openai-responses", func(_ llmdialogue.DialogueResult) { completed = true })
			if (err == nil) != tc.valid || completed != tc.valid {
				t.Fatalf("SSE 完成状态错误 completed=%v err=%v body=%s", completed, err, raw)
			}
			if !tc.valid && strings.Contains(string(raw), "response.completed") {
				t.Fatal("失败流不得输出成功终态")
			}
		})
	}
}
