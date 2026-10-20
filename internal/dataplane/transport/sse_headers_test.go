package transport

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestBypassSSECacheHeaders 验证原生SSE正常和错误响应都保留缓存限制并禁止代理重组；JSON不被修改。
// 前置内存供应商响应，分别覆盖WriteThrough错误正文与直接头复制，无外部服务和持久数据。
func TestBypassSSECacheHeaders(t *testing.T) {
	for _, status := range []int{200, 500} {
		for _, contentType := range []string{"text/event-stream; charset=utf-8", "application/json"} {
			t.Run(http.StatusText(status)+contentType, func(t *testing.T) {
				w := httptest.NewRecorder()
				WriteThrough(w, ForwardResponse{StatusCode: status, Header: http.Header{"Content-Type": {contentType}, "Cache-Control": {"private, no-store"}}, Body: []byte("event: error\ndata: {}\n\n")})
				want := "private, no-store"
				if contentType != "application/json" {
					want += ", no-transform"
				}
				if w.Code != status || w.Header().Get("Cache-Control") != want {
					t.Fatalf("原生状态/缓存头被改变: status=%d header=%v", w.Code, w.Header())
				}
			})
		}
	}
}
