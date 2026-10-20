package regression

import (
	"net/http"
	"strings"
	"testing"
)

// TestEditedNullPricesKeepStrictBudget 验证真实模型编辑PATCH清除空费率后，有限额Key仍能调用并按实测token结算。
// 前置私有PG和本地上游，覆盖创建、UI同形null更新、持久化、错误空表拒绝及数据面费用；删除模型和测试schema。
func TestEditedNullPricesKeepStrictBudget(t *testing.T) {
	t.Setenv("XHUB_REGRESSION_REDIS_URL", "")
	h := newHarness(t)
	admin := h.adminSession()
	name := "edited-null-prices"
	params := map[string]any{"model": name, "custom_llm_provider": "openai", "api_base": h.prices.URL + "/v1", "api_key": "sk-fake", "input_cost_per_token": 0.000001, "output_cost_per_token": 0.000002}
	created := h.ok(http.MethodPost, "/model/new", admin, map[string]any{"model_name": name, "litellm_params": params, "model_info": map[string]any{"transport": "bypass_openai_chat", "endpoint_types": []string{"chat"}, "pricing_source": "manual", "max_input_tokens": 1024, "max_output_tokens": 8192, "supported_parameters": []string{"max_tokens"}}}).json()
	id := created["model_info"].(map[string]any)["id"].(string)
	defer h.ok(http.MethodPost, "/model/delete", admin, map[string]any{"id": id})
	owner := h.openScope(t, admin, name)
	h.setKeyBudget(t, admin, owner.key, 1)
	patch := map[string]any{"model": name, "custom_llm_provider": "openai", "input_cost_per_token": 0.000001, "output_cost_per_token": 0.000002}
	for _, key := range strings.Fields("rates input_cost_per_token_peak output_cost_per_token_peak cache_read_input_token_cost cache_creation_input_token_cost cache_creation_input_token_cost_above_1hr input_cost_per_image output_cost_per_image input_cost_per_second output_cost_per_second search_context_cost_per_query") {
		patch[key] = nil
	}
	h.ok(http.MethodPatch, "/model/"+id+"/update", admin, map[string]any{"litellm_params": patch, "model_info": map[string]any{"pricing_source": "manual", "base_model": nil}})
	saved := h.ok(http.MethodGet, "/v2/model/info?modelId="+id, admin, nil).json()["data"].([]any)[0].(map[string]any)["litellm_params"].(map[string]any)
	if saved["input_cost_per_token"] != 0.000001 || saved["output_cost_per_token"] != 0.000002 || saved["rates"] != nil {
		t.Fatalf("编辑后单价未保留: %v", saved)
	}
	for _, bad := range []any{[]any{}, "bad", map[string]any{}} {
		got := h.do(http.MethodPatch, "/model/"+id+"/update", admin, map[string]any{"litellm_params": map[string]any{"rates": bad}})
		if got.status != 400 {
			t.Fatalf("非法非空值/空表未拒绝: %s", got.describe())
		}
	}
	for _, stream := range []bool{false, true} {
		got := h.ok(http.MethodPost, "/v1/chat/completions", owner.key, map[string]any{"model": name, "max_tokens": 64, "stream": stream, "messages": []any{map[string]any{"role": "user", "content": "Verify edited model billing."}}})
		if !strings.Contains(got.text(), "regression-ok") {
			t.Fatalf("编辑后无法对话: %s", got.describe())
		}
	}
	holds, err := h.db.PendingKeyBudget(t.Context(), owner.keyID)
	if err != nil || len(holds) != 0 {
		t.Fatalf("成功请求仍保留预占: %v %v", holds, err)
	}
	key, err := h.db.GetKey(t.Context(), owner.keyID)
	if err != nil || !nearlyEqual(key.Spend, 2*(11*0.000001+5*0.000002)) {
		t.Fatalf("实际token计费不符: %+v %v", key, err)
	}
}
