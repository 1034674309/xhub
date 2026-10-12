package usage

import "testing"

// TestActivityEntityTotalsIncludeUnassigned 验证已绑定、未绑定、无密钥及失败请求均守恒。
// 参数 t 为测试上下文；使用纯内存行，覆盖团队、组织、用户与无维度边界；无持久化，无需清理。
func TestActivityEntityTotalsIncludeUnassigned(t *testing.T) {
	for _, dim := range []entityDim{entityTeam, entityOrg, entityUser, entityNone} {
		rows := []activityRow{
			{day: "2026-10-12", teamID: "team", organizationID: "org", userID: "user", apiKey: "key", prompt: 10, completion: 2, spend: 0.5, success: true},
			{day: "2026-10-12", apiKey: "personal", prompt: 3, spend: 0.1, success: false},
			{day: "2026-10-12", prompt: 1, success: true},
		}
		body := activityBody(rows, 1, true, dim, entityLabels{})
		day := body["results"].([]any)[0].(map[string]any)
		entities := day["breakdown"].(map[string]any)["entities"].(map[string]any)
		if dim == entityNone {
			if len(entities) != 0 {
				t.Fatalf("无维度不得生成实体桶: %#v", entities)
			}
			continue
		}
		if _, ok := entities[""]; !ok {
			t.Fatalf("维度 %v 丢失未绑定桶: %#v", dim, entities)
		}
		requests, successes, failures, tokens := 0, 0, 0, 0
		spend := 0.0
		for _, entity := range entities {
			m := entity.(map[string]any)["metrics"].(map[string]any)
			requests += m["api_requests"].(int)
			successes += m["successful_requests"].(int)
			failures += m["failed_requests"].(int)
			tokens += m["total_tokens"].(int)
			spend += m["spend"].(float64)
		}
		if requests != 3 || successes != 2 || failures != 1 || tokens != 16 || spend != 0.6 {
			t.Fatalf("维度 %v 实体指标不守恒: 请求=%d 成功=%d 失败=%d token=%d 支出=%v", dim, requests, successes, failures, tokens, spend)
		}
	}
}
