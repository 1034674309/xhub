package measurement

import (
	"fmt"
	"testing"

	llmdialogue "github.com/sunqirui1987/xhub/internal/llm/dialogue"
)

// TestNativeStreamUsageRetainsConflicts 验证旁路中间帧的矛盾/非法数字不能被末帧清除；正常等价别名保持原值。
// 参数 t 为测试上下文，使用完整本地 SSE 字符串，覆盖三协议和缺失中间计量，无外部资源。
func TestNativeStreamUsageRetainsConflicts(t *testing.T) {
	for _, protocol := range []string{"openai-chat", "openai-responses", "anthropic-messages"} {
		t.Run(protocol, func(t *testing.T) {
			raw := []byte("data: {\"usage\":{\"prompt_tokens\":8,\"input_tokens\":9}}\n\ndata: {\"usage\":{\"prompt_tokens\":8,\"input_tokens\":8,\"completion_tokens\":2,\"pricing_blocked\":\"\"}}\n\n")
			facts := NativeStreamUsage(protocol, raw, nil)
			if facts["pricing_blocked"] != "upstream_usage_conflict" {
				t.Fatalf("旁路后续帧清除了冲突: %+v", facts)
			}
			valid := NativeStreamUsage(protocol, []byte("data: {\"usage\":{\"prompt_tokens\":8,\"input_tokens\":8,\"completion_tokens\":2}}\n\n"), nil)
			if valid["pricing_blocked"] != nil || valid["prompt_tokens"] != float64(8) {
				t.Fatalf("正常同义用量被错误拒绝: %+v", valid)
			}
		})
	}
}

// TestNativeStreamUsageCumulativeAliases 验证同批旁路事件合法增长时清除另一别名的旧快照；本地字符串，无外部清理。
// 三协议均覆盖两组别名的两种更新方向；终态归一化不得把跨帧变化当作同帧矛盾。
func TestNativeStreamUsageCumulativeAliases(t *testing.T) {
	for _, protocol := range []string{"openai-chat", "openai-responses", "anthropic-messages"} {
		for _, field := range []string{"prompt_tokens", "input_tokens", "completion_tokens", "output_tokens"} {
			t.Run(protocol+"/"+field, func(t *testing.T) {
				raw := fmt.Sprintf("data: {\"usage\":{\"prompt_tokens\":8,\"input_tokens\":8,\"completion_tokens\":0,\"output_tokens\":0}}\n\ndata: {\"usage\":{%q:9}}\n\n", field)
				got := llmdialogue.NormalizeDialogueUsage(NativeStreamUsage(protocol, []byte(raw), nil))
				if got["pricing_blocked"] != nil {
					t.Fatalf("合法累计更新被误认为冲突: %+v", got)
				}
			})
		}
	}
}

// TestNativeUsageSyncAliases 验证原生 JSON 仍只校验同一响应中的计数，不受流累计合并规则影响。
// 三协议使用等价/矛盾/非法计数对照；保持原始输入不变，无外部资源需要清理。
func TestNativeUsageSyncAliases(t *testing.T) {
	for _, protocol := range []string{"openai-chat", "openai-responses", "anthropic-messages"} {
		for _, tc := range []struct {
			name, blocked string
			input         any
		}{{"equal", "", 8}, {"conflict", "upstream_usage_conflict", 9}, {"fraction", "upstream_usage_invalid", 8.5}} {
			t.Run(protocol+"/"+tc.name, func(t *testing.T) {
				usage := map[string]any{"prompt_tokens": tc.input, "input_tokens": tc.input, "completion_tokens": 2}
				if tc.name == "conflict" {
					usage["prompt_tokens"] = 8
				}
				got := NativeUsage(protocol)(map[string]any{"usage": usage}, nil)
				reason, _ := got["pricing_blocked"].(string)
				if reason != tc.blocked || usage["pricing_blocked"] != nil {
					t.Fatalf("同步原始计数校验漂移: got=%+v source=%+v", got, usage)
				}
			})
		}
	}
}
