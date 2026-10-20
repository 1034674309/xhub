package keys

import (
	"github.com/sunqirui1987/xhub/internal/iam"
	"testing"
)

// TestKeyCapabilities 验证创建、编辑及轮换共用能力校验；前置 JSON 值，无数据库依赖或清理。
// 正常模型凭据、缺省与显式关闭可用；不支持类型、调度、宽限期及错误类型必须返回错误。
func TestKeyCapabilities(t *testing.T) {
	for _, body := range []map[string]any{{}, {"key_type": "llm_api", "allowed_routes": []any{"llm_api_routes"}, "auto_rotate": false}, {"key_type": "", "auto_rotate": nil, "rotation_interval": "", "grace_period": "", "allowed_routes": []any{}}} {
		if _, err := keyInputFrom(body, "me"); err != nil {
			t.Fatalf("有效声明被拒绝: %v %v", body, err)
		}
		if _, err := patchFrom(&iam.Key{}, body); err != nil {
			t.Fatalf("有效更新被拒绝: %v", err)
		}
	}
	for _, body := range []map[string]any{{"duration": map[string]any{}}, {"duration": 5}, {"key_type": "management"}, {"key_type": "default"}, {"key_type": "read_only"}, {"key_type": 42}, {"auto_rotate": true}, {"auto_rotate": "false"}, {"rotation_interval": "30d"}, {"rotation_interval": false}, {"grace_period": "60s"}, {"grace_period": map[string]any{}}, {"allowed_routes": []any{"management_routes"}}, {"allowed_routes": "llm_api_routes"}, {"allowed_routes": []any{true}}} {
		if _, err := keyInputFrom(body, "me"); err == nil {
			t.Fatalf("创建静默接受不支持声明: %v", body)
		}
		if _, err := patchFrom(&iam.Key{}, body); err == nil {
			t.Fatalf("更新静默接受不支持声明: %v", body)
		}
	}
	out := Response(iam.Key{}, "", false)
	if out["key_type"] != "llm_api" || out["auto_rotate"] != false || out["rotation_interval"] != nil {
		t.Fatalf("回读没有明确支持能力: %v", out)
	}
}
