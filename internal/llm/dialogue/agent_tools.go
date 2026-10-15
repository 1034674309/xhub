package dialogue

// 参照 LiteLLM 35970c3 的 namespace/custom 工具双向映射；MIT 来源见 third_party/litellm/NOTICE.md。
import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
)

// ToolBinding 记录 Chat 函数名对应的 Responses 工具身份；仅用于本请求的跨协议回复，不授予工具执行权限。
type ToolBinding struct {
	Name, Namespace string
	Custom          bool
}

// qualifiedToolName 为 namespace 函数构造稳定的 Chat 名称；参数为原命名空间和函数名，命名空间别名最长 64 字节，顶层名保持原值。
// 请求和历史投影共用；长名称追加摘要防止截断碰撞，原始身份通过 ToolBinding 恢复，无外部副作用。
func qualifiedToolName(namespace, name string) string {
	if namespace == "" {
		return name
	}
	joined := namespace + "__" + name
	if len(joined) <= 64 {
		return joined
	}
	return fmt.Sprintf("%s_%x", joined[:39], sha256.Sum256([]byte(joined)))[:64]
}

// projectAgentTools 将 Responses 函数、单层 namespace 和 custom 工具投影为共享函数，返回工具、还原表、不可桥接原因及输入错误。
// ParseDialogue 调用；未知原生工具保留在 Native，阻止桥接但允许原生路径；重复/碰撞名称明确拒绝，不静默覆盖工具身份。
// custom 的语法定义仅作为模型提示，不能等价提供原厂 grammar 强约束；本方法不执行工具或修改输入。
func projectAgentTools(values []any) ([]any, map[string]ToolBinding, string, error) {
	var projected []any
	bindings := map[string]ToolBinding{}
	seen := map[string]bool{}
	bridgeError := ""
	for _, value := range values {
		tool := dialogueMap(value)
		if tool == nil {
			return nil, nil, "", fmt.Errorf("tool must be an object")
		}
		namespace := ""
		members := []any{tool}
		if tool["type"] == "namespace" {
			if err := dialogueFields(tool, "type", "name", "description", "tools"); err != nil {
				bridgeError = "namespace " + err.Error()
			}
			namespace = dialogueString(tool["name"])
			var ok bool
			members, ok = tool["tools"].([]any)
			if namespace == "" || !ok {
				return nil, nil, "", fmt.Errorf("namespace requires name and tools array")
			}
		}
		for _, value := range members {
			member := dialogueMap(value)
			if member == nil {
				return nil, nil, "", fmt.Errorf("namespace tool must be an object")
			}
			kind := dialogueString(member["type"])
			if kind != "function" && kind != "custom" {
				bridgeError = "tool type " + kind
				continue
			}
			name := dialogueString(member["name"])
			if name == "" {
				return nil, nil, "", fmt.Errorf("tool name is required")
			}
			alias := qualifiedToolName(namespace, name)
			if seen[alias] {
				// 原生 namespace 与顶层名称可以合法共存；只拒绝有歧义的 Chat 候选，不能破坏原生部署。
				bridgeError = "conflicting tool name " + alias
			}
			seen[alias] = true
			cp, _ := cloneObject(member)
			delete(cp, "cache_control")
			cp["name"] = alias
			if kind == "custom" {
				// custom 会被重建成函数，必须先检查能力边界，避免未知格式或扩展在重建时静默消失；原生正文不受影响。
				if err := dialogueFields(member, "type", "name", "description", "format", "cache_control"); err != nil {
					bridgeError = "custom " + err.Error()
				}
				description := dialogueString(member["description"])
				if value := member["format"]; value != nil {
					format := dialogueMap(value)
					switch format["type"] {
					case "grammar":
						if err := dialogueFields(format, "type", "syntax", "definition"); err != nil || dialogueString(format["definition"]) == "" {
							bridgeError = "unsupported custom grammar format"
						}
						description += "\n\nInput format:\n" + dialogueString(format["definition"])
					case "text":
						if len(format) != 1 {
							bridgeError = "unsupported custom text format"
						}
					default:
						bridgeError = "unsupported custom format"
					}
				}
				cp = map[string]any{"type": "function", "name": alias, "description": description, "parameters": map[string]any{"type": "object", "properties": map[string]any{"content": map[string]any{"type": "string"}}, "required": []any{"content"}, "additionalProperties": false}}
			}
			if namespace != "" || kind == "custom" {
				bindings[alias] = ToolBinding{Name: name, Namespace: namespace, Custom: kind == "custom"}
			}
			projected = append(projected, cp)
		}
	}
	return projected, bindings, bridgeError, nil
}

// projectAgentCall 将带 namespace/custom 的 Responses 历史调用转为共享函数调用；参数为独立副本，返回输入格式错误。
// ParseDialogue 在原生正文以外的投影调用；公开工具字符串包装为 JSON，保留 call_id，既不执行也不重写用户输入字符串。
func projectAgentCall(item map[string]any) error {
	kind := dialogueString(item["type"])
	if kind == "custom_tool_call" {
		input, ok := item["input"].(string)
		if !ok {
			return fmt.Errorf("custom tool input must be a string")
		}
		raw, _ := json.Marshal(map[string]any{"content": input})
		item["type"], item["arguments"] = "function_call", string(raw)
		delete(item, "input")
	} else if kind == "custom_tool_call_output" {
		item["type"] = "function_call_output"
	}
	if item["type"] == "function_call" {
		item["name"] = qualifiedToolName(dialogueString(item["namespace"]), dialogueString(item["name"]))
		delete(item, "namespace")
	}
	// phase 是客户端显示阶段提示；正文与调用身份已经保留，Chat 没有独立的阶段字段。
	if phase := item["phase"]; phase != nil {
		if phase != "commentary" && phase != "final_answer" {
			return fmt.Errorf("unsupported message phase")
		}
		delete(item, "phase")
	}
	return nil
}

// ResponseToolItem 根据本请求的映射还原 Responses 工具项；参数为完整函数调用、映射和生命周期状态，返回项或非法 custom 参数错误。
// JSON 及 SSE 编码器共用；custom 参数必须有字符串 content，不把错误 JSON 当成可执行输入；函数无外部副作用。
func ResponseToolItem(call Turn, bindings map[string]ToolBinding, status string) (map[string]any, error) {
	item := map[string]any{"id": call.ID, "type": "function_call", "call_id": call.ID, "name": call.Name, "arguments": call.Arguments, "status": status}
	binding, ok := bindings[call.Name]
	if !ok {
		return item, nil
	}
	item["name"] = binding.Name
	if binding.Namespace != "" {
		item["namespace"] = binding.Namespace
	}
	if binding.Custom {
		var args map[string]any
		if json.Unmarshal([]byte(call.Arguments), &args) != nil {
			return nil, fmt.Errorf("invalid custom tool arguments")
		}
		content, valid := args["content"].(string)
		if !valid || len(args) != 1 {
			return nil, fmt.Errorf("custom tool requires a content string")
		}
		item["type"], item["input"] = "custom_tool_call", content
		delete(item, "arguments")
	}
	return item, nil
}

// agentClientMetadata 验证 Codex 的客户端跟踪元数据；参数为任意字段，返回类型/大小错误。
// 跨协议选项投影调用；只丢弃不影响推理语义的跟踪提示，不能用它覆盖网关身份、权限、会话或预算。
func agentClientMetadata(value any) error {
	m, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("client_metadata must be an object")
	}
	for _, value := range m {
		if _, ok := value.(string); !ok {
			return fmt.Errorf("client_metadata values must be strings")
		}
	}
	raw, _ := json.Marshal(m)
	if len(raw) > 64<<10 {
		return fmt.Errorf("client_metadata is too large")
	}
	return nil
}

// validReasoningSummary 判断 Responses 摘要提示是否为已知枚举；参数为任意 JSON 值，返回合法性，无副作用。
// bridgeOptions 使用；这些提示不改变推理 effort，Chat 已保留公开 reasoning_content，但不承诺原生摘要粒度。
func validReasoningSummary(value any) bool {
	s, ok := value.(string)
	return ok && strings.Contains("|auto|concise|detailed|none|", "|"+s+"|")
}

// projectReasoningContext 将 Responses 的公开推理保留范围投影到 Chat 历史；参数为已有轮次和 context 提示，返回独立切片或枚举错误。
// EncodeDialogue 调用；current_turn 仅移除最后一条用户消息之前的公开推理，保留正文、工具和关联 ID；auto/all_turns 沿用完整可用历史。
// 不解释厂商密文，也不修改原轮次；不透明推理在外层能力检查中已经拒绝跨协议。
func projectReasoningContext(turns []Turn, context any) ([]Turn, error) {
	if context == nil || context == "auto" || context == "all_turns" {
		return append([]Turn(nil), turns...), nil
	}
	if context != "current_turn" {
		return nil, fmt.Errorf("invalid reasoning context")
	}
	lastUser := -1
	for index, turn := range turns {
		if turn.Role == "user" {
			lastUser = index
		}
	}
	var projected []Turn
	for index, turn := range turns {
		if index < lastUser {
			if turn.Role == "reasoning" {
				continue
			}
			turn.Reasoning = ""
		}
		projected = append(projected, turn)
	}
	return projected, nil
}
