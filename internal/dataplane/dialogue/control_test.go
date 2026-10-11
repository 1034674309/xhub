package dialogue

import (
	"testing"
)

// TestStripGatewayFields 验证仅删除网关控制参数，保留合法协议与厂商扩展；前置普通、空和 nil 正文。
// 参数 t 为单测上下文；返回无，失败指出泄露或误删字段；仅内存数据无需清理。
func TestStripGatewayFields(t *testing.T) {
	for _, body := range []map[string]any{nil, {}, {
		"api_key": "secret", "api_base": "https://invalid.example", "disable_fallbacks": true, "guardrails": []string{"redact"},
		"metadata": map[string]any{"request": "test"}, "id": "item", "user": "user", "cache": true, "vendor_extension": "keep",
	}} {
		StripGatewayFields(body)
		for _, key := range []string{"api_key", "api_base", "disable_fallbacks", "guardrails"} {
			if _, ok := body[key]; ok {
				t.Fatalf("控制参数 %s 泄露到上游", key)
			}
		}
		if len(body) > 0 {
			for _, key := range []string{"metadata", "id", "user", "cache", "vendor_extension"} {
				if _, ok := body[key]; !ok {
					t.Fatalf("合法参数 %s 被误删", key)
				}
			}
		}
	}
}
