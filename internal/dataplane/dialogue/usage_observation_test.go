package dialogue

import "testing"

// TestStreamUsageConflictSticky 验证流入口在覆盖别名前保存冲突，后续终态不能清除；正常增量仍以最后实测为准。
// 纯内存状态机，覆盖输入/输出别名和非法中间值，无网络、数据库或清理动作。
func TestStreamUsageConflictSticky(t *testing.T) {
	for _, usage := range []map[string]any{
		{"prompt_tokens": 1024, "input_tokens": 11},
		{"completion_tokens": 10, "output_tokens": 5},
		{"input_tokens": "11"},
	} {
		s := dialogueStream{usage: map[string]any{}}
		s.observeUsage(usage)
		s.observeUsage(map[string]any{"input_tokens": 11, "output_tokens": 5, "pricing_blocked": ""})
		if reason, _ := s.usage["pricing_blocked"].(string); reason == "" {
			t.Fatalf("中间帧矛盾被终态用量覆盖: %+v", s.usage)
		}
	}
	s := dialogueStream{usage: map[string]any{}}
	s.observeUsage(map[string]any{"input_tokens": 11, "output_tokens": 0})
	s.observeUsage(map[string]any{"output_tokens": 5})
	if s.usage["prompt_tokens"] != 11 || s.usage["completion_tokens"] != 5 || s.usage["pricing_blocked"] != nil {
		t.Fatalf("合法部分用量未被正确合并: %+v", s.usage)
	}
}
