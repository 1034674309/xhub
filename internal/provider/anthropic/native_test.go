package anthropic

import (
	"testing"

	"github.com/sunqirui1987/xhub/internal/provider/registry"
)

// TestMessagesRegistration 验证 Messages 路径、API key 和版本头；前置本包初始化，非法方法与路径拒绝。
// 参数 t 为单测上下文，返回无；只读内置注册表，无外部资源或清理。
func TestMessagesRegistration(t *testing.T) {
	hit, ok := registry.Match("POST", "/bypass/anthropic/v1/messages")
	if !ok || hit.Transport.Auth.Header != "x-api-key" || hit.Transport.Headers["anthropic-version"] != "2023-06-01" || hit.Action.UpstreamPath != "/v1/messages" {
		t.Fatalf("Messages 登记错误: %+v", hit)
	}
	for _, path := range []string{"/bypass/anthropic/v1/unknown", "/bypass/anthropic/v1/messages/extra"} {
		if _, ok := registry.Match("POST", path); ok {
			t.Fatalf("未知路径被接受: %s", path)
		}
	}
	if _, ok := registry.Match("GET", "/bypass/anthropic/v1/messages"); ok {
		t.Fatal("非法方法被接受")
	}
}
