package billing

import (
	"encoding/json"
	"github.com/sunqirui1987/xhub/internal/catalog"
	"testing"
	"time"
)

// TestStrictBudgetNullPriceClearing 验证模型编辑器的null表示清除价格而非非法表或隐式免费。
// 前置纯内存完整单价部署；正常null保留真实收费，缺少必需价/空数组/非法表仍拒绝，输入不修改，无外部清理。
func TestStrictBudgetNullPriceClearing(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fields map[string]any
		valid  bool
	}{
		{"editor_nulls", map[string]any{"rates": nil, "input_cost_per_token_peak": nil, "output_cost_per_token_peak": nil, "cache_read_input_token_cost": nil, "cache_creation_input_token_cost": nil, "cache_creation_input_token_cost_above_1hr": nil, "input_cost_per_image": nil, "output_cost_per_image": nil, "input_cost_per_second": nil, "output_cost_per_second": nil, "search_context_cost_per_query": nil}, true},
		{"input_cleared", map[string]any{"rates": nil, "input_cost_per_token": nil}, false},
		{"output_cleared", map[string]any{"rates": nil, "output_cost_per_token": nil}, false},
		{"empty_array", map[string]any{"rates": []any{}}, false},
		{"object", map[string]any{"rates": map[string]any{}}, false},
		{"string", map[string]any{"rates": ""}, false},
		{"flat_wrong_type", map[string]any{"rates": nil, "cache_read_input_token_cost": ""}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dep := strictBudgetDeployment()
			for k, v := range tc.fields {
				dep.LiteLLMParams[k] = v
			}
			before, _ := json.Marshal(dep)
			body := strictBudgetBody("openai-chat")
			body["max_tokens"] = 64
			plan, _, err := StrictBudgetPlan(dep, strictBudgetEncode(t, body), "openai-chat", time.Now())
			if !tc.valid {
				strictBudgetExpectError(t, err, "unsupported_budget")
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			charge, err := plan.Charge(catalog.Usage{PromptTokens: 8, CompletionTokens: 2})
			if err != nil || charge.Total != 0.000012 {
				t.Fatalf("null清除后实际计费错误: %+v %v", charge, err)
			}
			rates, err := budgetRates(dep, nil)
			if err != nil || len(rates) != 2 {
				t.Fatalf("null缓存价不能生成免费费率: %+v %v", rates, err)
			}
			after, _ := json.Marshal(dep)
			if string(before) != string(after) {
				t.Fatal("计划生成修改了持久化部署")
			}
		})
	}
}
