package billing

import (
	"testing"
	"time"

	"github.com/sunqirui1987/xhub/internal/catalog"
	"github.com/sunqirui1987/xhub/internal/config"
	llmdialogue "github.com/sunqirui1987/xhub/internal/llm/dialogue"
)

// TestStrictBudgetCacheUsageAliases 验证有缓存价格时仍拒绝缓存别名矛盾，并区分合法的独立缓存计数和总量子集。
// 前置冻结手动价格；原始 JSON 及预先归一化两条计费路径行为一致，纯内存无需外部清理。
func TestStrictBudgetCacheUsageAliases(t *testing.T) {
	dep := config.ModelEntry{
		ModelName: "cache-usage",
		LiteLLMParams: map[string]any{
			"model": "cache-usage", "input_cost_per_token": 1, "output_cost_per_token": 1,
			"cache_read_input_token_cost": 0.1, "cache_creation_input_token_cost": 0.5,
		},
		ModelInfo: map[string]any{"pricing_source": "manual", "max_input_tokens": 100, "supported_parameters": []string{"max_completion_tokens"}},
	}
	plan, _, err := StrictBudgetPlan(dep, []byte(`{"messages":[{"role":"user","content":"Verify cache usage."}],"max_completion_tokens":10}`), "openai-chat", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		raw     map[string]any
		blocked bool
		cost    float64
	}{
		{"write_conflict", map[string]any{"prompt_tokens": 10, "completion_tokens": 1, "cache_creation_input_tokens": 0, "cache_write_tokens": 50}, true, 0},
		{"read_conflict", map[string]any{"prompt_tokens": 10, "completion_tokens": 1, "cached_tokens": 0, "prompt_tokens_details": map[string]any{"cached_tokens": 8}}, true, 0},
		{"read_alias_conflict", map[string]any{"input_tokens": 10, "output_tokens": 1, "cache_read_input_tokens": 0, "cache_read_tokens": 8}, true, 0},
		{"zero", map[string]any{"prompt_tokens": 10, "completion_tokens": 1, "cache_creation_input_tokens": 0, "cache_write_tokens": 0}, false, 11},
		{"read_subset", map[string]any{"prompt_tokens": 10, "completion_tokens": 1, "cached_tokens": 8, "prompt_tokens_details": map[string]any{"cached_tokens": 8}}, false, 3.8},
		{"read_separate", map[string]any{"input_tokens": 10, "output_tokens": 1, "cache_read_input_tokens": 8, "cache_read_tokens": 8}, false, 11.8},
		{"write_equal", map[string]any{"prompt_tokens": 10, "completion_tokens": 1, "cache_creation_input_tokens": 8, "cache_write_tokens": 8}, false, 15},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, raw := range []map[string]any{tc.raw, llmdialogue.NormalizeDialogueUsage(tc.raw)} {
				err := ValidateStrictBudgetUsage(raw, plan)
				if (err != nil) != tc.blocked {
					t.Fatalf("缓存别名校验错误: raw=%+v err=%v", raw, err)
				}
				if tc.blocked {
					continue
				}
				charge, err := plan.Charge(catalog.NormalizeUsage(llmdialogue.NormalizeDialogueUsage(raw)))
				if err != nil || charge.Total-tc.cost > 1e-9 || tc.cost-charge.Total > 1e-9 {
					t.Fatalf("缓存子集/独立计数被重复计算或漏计: charge=%+v want=%v err=%v", charge, tc.cost, err)
				}
			}
		})
	}
}
