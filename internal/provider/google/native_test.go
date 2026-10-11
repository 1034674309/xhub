package google

import (
	"testing"

	"github.com/sunqirui1987/xhub/internal/provider/registry"
)

// TestGoogleRegistration 验证两种 Google 鉴权及带斜杠模型匹配，错误动作和方法拒绝；前置本包初始化。
// 参数 t 为单测上下文，返回无；只读内置注册表，无外部资源或清理。
func TestGoogleRegistration(t *testing.T) {
	for _, tc := range []struct{ path, header, prefix string }{{"/v1beta/models/org/model:generateContent", "x-goog-api-key", ""}, {"/vertex/v1/models/org/model:generateContent", "Authorization", "Bearer"}} {
		hit, ok := registry.Match("POST", tc.path)
		if !ok || hit.Names["model"] != "org/model" || hit.Transport.Auth.Header != tc.header || hit.Transport.Auth.Prefix != tc.prefix {
			t.Fatalf("Google 登记错误: %+v", hit)
		}
		if _, ok := registry.Match("GET", tc.path); ok {
			t.Fatal("非法方法被接受")
		}
	}
	if _, ok := registry.Match("POST", "/v1beta/models/model:unknown"); ok {
		t.Fatal("未知动作被接受")
	}
}
