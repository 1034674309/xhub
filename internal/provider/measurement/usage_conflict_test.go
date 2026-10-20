package measurement

import "testing"

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
