package stream

import (
	"testing"
)

// TestResponseIDProtocols 验证 JSON/SSE 正常、坏帧和输出项归属；参数 t 为测试上下文。
// 只使用内存，必须返回响应 ID 且忽略输出项，无外部数据需要清理。
func TestResponseIDProtocols(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{"{\"id\":\"chatcmpl-a\"}", "chatcmpl-a"},
		{"data: {\"type\":\"response.output_item.added\",\"id\":\"item-a\"}\n\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp-a\"}}\n\n", "resp-a"},
		{"data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg-a\"}}\n", "msg-a"},
		{"data: broken\ndata: {\"id\":\"\"}\ndata: [DONE]\n", ""},
		{"{\"type\":\"response.created\",\"response\":{\"id\":12}}", ""},
		{"data: {\"id\":\"chatcmpl-b\"}\n", "chatcmpl-b"},
	} {
		if got := ResponseID([]byte(tc.raw)); got != tc.want {
			t.Fatalf("归属 ID raw=%q got=%q want=%q", tc.raw, got, tc.want)
		}
	}
}
