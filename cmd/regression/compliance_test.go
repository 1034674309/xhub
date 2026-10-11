package regression

import (
	"testing"
)

// TestComplianceModuleContract 前置真实网关和隔离 schema，验证模块挂载、鉴权、请求校验及完整检查数组。
// 参数 t 为测试上下文；没有外部调用，newHarness 的 Cleanup 删除全部私有数据。
func TestComplianceModuleContract(t *testing.T) {
	h := newHarness(t)
	admin := h.adminSession()
	for _, tc := range []struct{ path, regulation string }{{"/compliance/eu-ai-act", "EU AI Act"}, {"/compliance/gdpr", "GDPR"}} {
		if got := h.do("POST", tc.path, "", map[string]any{"request_id": "r"}); got.status != 401 {
			t.Fatalf("合规接口鉴权错误: %s", got.describe())
		}
		if got := h.do("POST", tc.path, admin, map[string]any{}); got.status != 400 {
			t.Fatalf("缺请求标识应拒绝: %s", got.describe())
		}
		result := h.ok("POST", tc.path, admin, map[string]any{"request_id": "r"}).json()
		checks, ok := result["checks"].([]any)
		if !ok || len(checks) != 3 || result["compliant"] != false || result["regulation"] != tc.regulation {
			t.Fatalf("合规契约错误: %v", result)
		}
		result = h.ok("POST", tc.path, admin, map[string]any{"request_id": "r", "user_id": "u", "model": "public", "timestamp": "2026-10-11T00:00:00Z", "guardrail_information": []any{map[string]any{"guardrail_mode": "pre_call", "guardrail_status": "success"}}}).json()
		if result["compliant"] != true {
			t.Fatalf("完整合规请求失败: %v", result)
		}
	}
}
