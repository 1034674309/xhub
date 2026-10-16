package dialogue

import "fmt"

// ChatReasoningItems 将已解析 Responses 的推理投影为 LiteLLM 风格的 Chat reasoning_items 扩展。
// 参数 result 为上游结果，返回独立的推理数组或字段错误；同步编码和流式终态共用，不修改原结果。
// 公开摘要仍由 reasoning_content 展示；密文只留在结构化扩展中，客户端可原样回传，也可按普通 Chat 忽略它。
func ChatReasoningItems(result DialogueResult) ([]any, error) {
	if result.Source != "openai-responses" {
		return nil, nil
	}
	var items []any
	for _, value := range dialogueList(result.Native["output"]) {
		item := dialogueMap(value)
		if item["type"] != "reasoning" {
			continue
		}
		copy, err := cloneObject(item)
		if err != nil {
			return nil, err
		}
		// 与 LiteLLM 的扩展形状保持一致，不把供应商新增的私有输出字段当成可重放输入。
		projected := map[string]any{"type": "reasoning", "id": copy["id"], "summary": []any{}}
		if summary, exists := copy["summary"]; exists {
			projected["summary"] = summary
		}
		if cipher, exists := copy["encrypted_content"]; exists {
			projected["encrypted_content"] = cipher
		}
		if _, err := chatReasoningReplay([]any{projected}, "assistant"); err != nil {
			return nil, err
		}
		items = append(items, projected)
	}
	return items, nil
}

// chatReasoningReplay 解析 Chat assistant 上回传的 reasoning_items，供共享解析器恢复 Responses 历史。
// 参数 value 为可选扩展、role 为消息角色，返回独立原生 Turn 或格式错误；空扩展不影响普通 Chat。
// 仅保留协议允许的字段，不解密、不拼接正文；原生 Turn 会阻止发送给 Chat/Anthropic 等其他上游，连接归属另由路由层验证。
func chatReasoningReplay(value any, role string) ([]Turn, error) {
	if value == nil {
		return nil, nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("reasoning_items must be an array")
	}
	if len(items) > 0 && role != "assistant" {
		return nil, fmt.Errorf("reasoning_items requires assistant role")
	}
	var turns []Turn
	for _, value := range items {
		item := dialogueMap(value)
		if err := dialogueFields(item, "type", "id", "summary", "encrypted_content", "status"); err != nil {
			return nil, err
		}
		if item["type"] != "reasoning" || dialogueString(item["id"]) == "" {
			return nil, fmt.Errorf("reasoning_items requires reasoning type and ID")
		}
		if _, err := MessagesReasoningBlock(item); err != nil {
			return nil, err
		}
		copy, err := cloneObject(item)
		if err != nil {
			return nil, err
		}
		// status 是输出生命周期字段；续轮使用原厂 ID、摘要和密文，不能伪造新的 reasoning ID。
		delete(copy, "status")
		turns = append(turns, Turn{Role: "reasoning", NativeProtocol: "openai-responses", Native: copy})
	}
	return turns, nil
}
