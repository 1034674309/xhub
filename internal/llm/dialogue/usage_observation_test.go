package dialogue

import (
	"encoding/json"
	"math"
	"testing"
)

// TestMergeDialogueUsage 验证完整/部分/空帧、等价别名、矛盾别名及非法计数；纯内存，无需外部清理。
// 后续合法终态及显式空标记不能消除先前发现的矛盾；输入 map 保持不变，正常部分帧仍允许补齐。
func TestMergeDialogueUsage(t *testing.T) {
	for _, tc := range []struct {
		name    string
		usage   map[string]any
		blocked bool
	}{
		{"nil", nil, false},
		{"partial", map[string]any{"input_tokens": 11}, false},
		{"equal_aliases", map[string]any{"prompt_tokens": 11, "input_tokens": json.Number("11")}, false},
		{"input_conflict", map[string]any{"prompt_tokens": 1024, "input_tokens": 11}, true},
		{"output_conflict", map[string]any{"completion_tokens": 1024, "output_tokens": 5}, true},
		{"string", map[string]any{"input_tokens": "11"}, true},
		{"fraction", map[string]any{"input_tokens": 1.5}, true},
		{"negative", map[string]any{"input_tokens": -1}, true},
		{"nan", map[string]any{"input_tokens": math.NaN()}, true},
		{"infinite", map[string]any{"input_tokens": math.Inf(1)}, true},
		{"null_partial", map[string]any{"input_tokens": nil}, false},
		{"nested", map[string]any{"output_tokens_details": map[string]any{"reasoning_tokens": "3"}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := MergeDialogueUsage(nil, tc.usage)
			got = MergeDialogueUsage(got, map[string]any{"prompt_tokens": 11, "input_tokens": 11, "completion_tokens": 5, "output_tokens": 5, "pricing_blocked": ""})
			got = NormalizeDialogueUsage(got)
			blocked, _ := got["pricing_blocked"].(string)
			if (blocked != "") != tc.blocked {
				t.Fatalf("先前用量信任状态被覆盖: %+v", got)
			}
			if _, changed := tc.usage["pricing_blocked"]; changed {
				t.Fatal("修改了供应商原始用量对象")
			}
		})
	}
}

// TestNormalizeDialogueUsageBeforeAliasReplacement 验证同步归一化先识别原始别名冲突，不能覆盖后再假装完整；纯内存自动清理。
func TestNormalizeDialogueUsageBeforeAliasReplacement(t *testing.T) {
	got := NormalizeDialogueUsage(map[string]any{"prompt_tokens": 1024, "input_tokens": 11, "completion_tokens": 5})
	if got["pricing_blocked"] != "upstream_usage_conflict" {
		t.Fatalf("归一化丢失原始别名冲突: %+v", got)
	}
}
