package upstreamurl

import (
	"testing"
)

// TestPassthroughURL 验证显式前缀、尾斜杠、官方 v1 补齐及非法 URL 降级；前置纯地址字符串。
// 参数 t 为单测上下文，返回无；不访问外部地址，无资源需清理。
func TestPassthroughURL(t *testing.T) {
	for _, tc := range []struct{ base, endpoint, protocol, want string }{{"https://api.openai.com", "chat/completions", "openai", "https://api.openai.com/v1/chat/completions"}, {"https://relay.example/root", "/v1/images/", "openai", "https://relay.example/root/v1/images/"}, {"https://relay.example/root", "../../escape", "openai", "https://relay.example/root/"}, {"https://relay.example/root?x=1", "tasks", "ark", "https://relay.example/root/tasks?x=1"}, {":invalid", "tasks", "openai", ":invalid"}, {"https://relay.example", "", "openai", "https://relay.example/"}} {
		if got := PassthroughURL(tc.base, tc.endpoint, tc.protocol); got != tc.want {
			t.Fatalf("拼接 %q + %q: got=%q want=%q", tc.base, tc.endpoint, got, tc.want)
		}
	}
}
