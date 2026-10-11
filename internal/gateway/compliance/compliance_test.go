package compliance

import (
	"testing"
)

// compliantPayload 构造完整审计和成功前置护栏记录；无参数，返回独立对象，合规单测调用，无副作用。
func compliantPayload() map[string]any {
	return map[string]any{
		"request_id": "req-1",
		"user_id":    "user-1",
		"model":      "gpt-4o",
		"timestamp":  "2026-10-08T00:00:00Z",
		"guardrail_information": []any{
			map[string]any{"guardrail_name": "pii", "guardrail_mode": "pre_call", "guardrail_status": "success"},
			map[string]any{"guardrail_name": "skipped", "guardrail_status": "not_run"},
		},
	}
}

// TestEUAIActWithoutGuardrailsFailsClosed 前置缺失护栏和审计的请求，验证全部检查失败且明确缺失字段。参数 t 为测试上下文；无外部数据。
func TestEUAIActWithoutGuardrailsFailsClosed(t *testing.T) {
	checks := euAIActChecks(map[string]any{"request_id": "req-1"})
	if len(checks) != 3 {
		t.Fatalf("checks = %d", len(checks))
	}
	for _, check := range checks {
		if check.Passed {
			t.Fatalf("expected failure, got %+v", check)
		}
	}
	if checks[2].Detail != "Missing: user_id, model, timestamp, guardrail_results" {
		t.Fatalf("audit detail = %q", checks[2].Detail)
	}
}

// TestCompliancePassesWhenPreCallSucceeds 前置完整审计与成功前置护栏，验证两类规则通过。参数 t 为测试上下文；无外部数据。
func TestCompliancePassesWhenPreCallSucceeds(t *testing.T) {
	body := compliantPayload()
	for _, check := range euAIActChecks(body) {
		if !check.Passed {
			t.Fatalf("eu check failed: %+v", check)
		}
	}
	for _, check := range gdprChecks(body) {
		if !check.Passed {
			t.Fatalf("gdpr check failed: %+v", check)
		}
	}
	if gdprChecks(body)[1].Detail != "No sensitive data detected" {
		t.Fatalf("sensitive detail = %q", gdprChecks(body)[1].Detail)
	}
}

// TestMixedGuardrailModeDoesNotCountAsPreCall 前置混合执行模式及被拦截记录，验证不能误记成功前置筛查，干预仍算数据保护。参数 t 为测试上下文；内存夹具无需清理。
func TestMixedGuardrailModeDoesNotCountAsPreCall(t *testing.T) {
	body := compliantPayload()
	body["guardrail_information"] = []any{
		map[string]any{"guardrail_mode": []any{"pre_call", "post_call"}, "guardrail_status": "success"},
	}
	screened := euAIActChecks(body)[1]
	if screened.Passed {
		t.Fatalf("mixed mode counted as pre-call: %+v", screened)
	}

	body["guardrail_information"] = []any{
		map[string]any{"guardrail_mode": []any{"pre_call"}, "guardrail_status": "blocked"},
	}
	if !euAIActChecks(body)[1].Passed {
		t.Fatal("single-mode list was not counted as pre-call")
	}
	sensitive := gdprChecks(body)[1]
	if !sensitive.Passed || sensitive.Detail != "Guardrail intervened to protect sensitive data" {
		t.Fatalf("intervention = %+v", sensitive)
	}
}

// TestSingleGuardrailObjectIsAccepted 前置单对象护栏记录，验证该实际日志形状被读取。参数 t 为测试上下文；内存夹具无需清理。
func TestSingleGuardrailObjectIsAccepted(t *testing.T) {
	body := compliantPayload()
	body["guardrail_information"] = map[string]any{
		"guardrail_mode":   "pre_call",
		"guardrail_status": "success",
	}
	if got := len(activeGuardrails(body)); got != 1 {
		t.Fatalf("active = %d", got)
	}
}
