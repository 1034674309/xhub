package measurement

import (
	"encoding/json"
	"strings"

	llmdialogue "github.com/sunqirui1987/xhub/internal/llm/dialogue"
)

// nativeResponsesTerminal 校验可结束原生传输的完整 Responses 快照，不改写任何客户端字段。
// 参数 response 为响应对象、status 为事件声明的终态；返回是否具有身份、匹配状态和合法输出。
// NativeUsage 仅用它放行明确 max_output_tokens，流观察器也用它判断 completed 能否提前关连接。
// 不接受未知截断原因或错误字段类型；明确长度截断可留下未闭合的参数字符串，但不把它改为可执行工具。
// 用量完整性及费用另由 NativeUsage 和硬额度核价检查，函数不修改原始快照。
func nativeResponsesTerminal(response map[string]any, status string) bool {
	id, _ := response["id"].(string)
	if strings.TrimSpace(id) == "" || response["error"] != nil || response["status"] != status {
		return false
	}
	if status == "incomplete" {
		details, _ := response["incomplete_details"].(map[string]any)
		if details["reason"] != "max_output_tokens" {
			return false
		}
	} else if status != "completed" {
		return false
	}
	raw, err := json.Marshal(response)
	if err != nil {
		return false
	}
	if status == "incomplete" {
		// cap 会合法地截断工具参数。只在临时校验副本中补一个有效参数占位，
		// 继续验证输出结构/函数身份；真实参数与 incomplete 原样发给客户端。
		var projection map[string]any
		if json.Unmarshal(raw, &projection) != nil {
			return false
		}
		items, _ := projection["output"].([]any)
		for _, value := range items {
			item, _ := value.(map[string]any)
			if item["type"] == "function_call" {
				arguments, ok := item["arguments"].(string)
				if !ok {
					return false
				}
				if !json.Valid([]byte(arguments)) {
					item["arguments"] = "{}"
				}
			}
		}
		raw, err = json.Marshal(projection)
		if err != nil {
			return false
		}
	}
	_, err = llmdialogue.ParseDialogueResult("openai-responses", raw)
	return err == nil
}
