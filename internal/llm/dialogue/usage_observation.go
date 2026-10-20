package dialogue

import (
	"math"
	"strings"
)

// MergeDialogueUsage 合并一帧原始实测用量，并保留任何已观察到的矛盾或非法计数。
// 参数 previous 为本次请求累积对象（可为空），incoming 为当前帧；返回累积对象，更新同名字段但不转换别名。
// 流适配器在归一化前调用；后续完整帧或空 pricing_blocked 不得把已不可信的费用重新标为可结算。
// 不拒绝缺少字段的合法部分帧，不修改 incoming，不执行计费或外部操作。
func MergeDialogueUsage(previous, incoming map[string]any) map[string]any {
	if previous == nil {
		previous = map[string]any{}
	}
	blocked, _ := previous["pricing_blocked"].(string)
	if blocked == "" {
		blocked = dialogueUsageProblem(incoming)
	}
	for key, value := range incoming {
		previous[key] = value
	}
	if blocked != "" {
		previous["pricing_blocked"] = blocked
	}
	return previous
}

// dialogueUsageProblem 检查归一化会覆盖的别名以及原始 token 数值；参数可为部分帧，返回机器可读原因或空串。
// 同一帧的等价别名必须一致，已报告的 token 必须为非负有限整数；null 仍视为缺失，最终完整性检查负责拒绝。
// 递归检查明细，避免非法值被后续帧覆盖后失去证据。
// 此处只标记已观察到的矛盾，不要求完整 usage；完整性和收费维度由最终计费契约验证。
func dialogueUsageProblem(usage map[string]any) string {
	for _, pair := range [][2]string{{"prompt_tokens", "input_tokens"}, {"completion_tokens", "output_tokens"}} {
		left, hasLeft := usage[pair[0]]
		right, hasRight := usage[pair[1]]
		if hasLeft && hasRight {
			a, okA := dialogueNumber(left)
			b, okB := dialogueNumber(right)
			if !okA || !okB || a != b {
				return "upstream_usage_conflict"
			}
		}
	}
	for key, value := range usage {
		if value == nil {
			continue
		}
		if strings.HasSuffix(key, "_tokens") {
			n, ok := dialogueNumber(value)
			if !ok || n < 0 || n != math.Trunc(n) {
				return "upstream_usage_invalid"
			}
		}
		if nested, ok := value.(map[string]any); ok {
			if problem := dialogueUsageProblem(nested); problem != "" {
				return problem
			}
		}
	}
	return ""
}
