package dialogue

import (
	"encoding/json"
	"math"
)

// dialogueNumber 读取有限数值；参数为动态数值，返回数和有效标记；请求校验调用，无副作用。
func dialogueNumber(v any) (float64, bool) {
	var n float64
	switch value := v.(type) {
	case float64:
		n = value
	case int:
		n = float64(value)
	case json.Number:
		var err error
		n, err = value.Float64()
		if err != nil {
			return 0, false
		}
	default:
		return 0, false
	}
	return n, !math.IsNaN(n) && !math.IsInf(n, 0)
}

// NormalizeDialogueUsage 统一实测用量名称并保留缓存事实；参数为协议用量，返回新对象。
// 调用：同步和流适配器；缺失实测输入或输出时标记禁止自动计价，不估算提示词。
func NormalizeDialogueUsage(u map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range u {
		out[key] = value
	}
	for source, target := range map[string]string{"input_tokens": "prompt_tokens", "output_tokens": "completion_tokens", "input_tokens_details": "prompt_tokens_details", "output_tokens_details": "completion_tokens_details"} {
		if value, exists := u[source]; exists {
			out[target] = value
		}
		delete(out, source)
	}
	if value, exists := u["cache_read_input_tokens"]; exists {
		out["prompt_tokens_details"] = map[string]any{"cached_tokens": value}
	}
	input, hasInput := dialogueNumber(out["prompt_tokens"])
	output, hasOutput := dialogueNumber(out["completion_tokens"])
	if !hasInput || !hasOutput {
		out["pricing_blocked"] = "upstream_usage_missing"
	} else {
		out["total_tokens"] = input + output
	}
	return out
}

// dialogueOutputUsage 将共享实测事实映射到用户用量协议；参数为事实与协议，返回新对象。
// 调用：同步输出；未知用量通过 pricing_blocked 明示，不生成估算数值。
func dialogueOutputUsage(u map[string]any, protocol string) map[string]any {
	out := map[string]any{}
	for key, value := range u {
		out[key] = value
	}
	for source, target := range map[string]string{"prompt_tokens": "input_tokens", "completion_tokens": "output_tokens", "prompt_tokens_details": "input_tokens_details", "completion_tokens_details": "output_tokens_details"} {
		if value, exists := u[source]; exists {
			out[target] = value
		}
		delete(out, source)
	}
	if protocol == "anthropic-messages" {
		if details := dialogueMap(u["prompt_tokens_details"]); details != nil {
			out["cache_read_input_tokens"] = details["cached_tokens"]
		}
		delete(out, "input_tokens_details")
		delete(out, "output_tokens_details")
		delete(out, "total_tokens")
	}
	return out
}
