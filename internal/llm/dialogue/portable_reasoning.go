package dialogue

import "strings"

// PortableReasoningItems 为统一 Responses 入口生成可迁移的历史副本，参数为 input/output 数组。
// 返回新数组及推理项副本，移除 reasoning 的账户绑定密文、来源 ID 和输出状态，保留摘要、正文、工具及其 call_id。
// Serve 在选路前以及 JSON/SSE 发布前调用；非数组保持原值，非 reasoning 项不改写，非法形状仍由协议解析器拒绝。
// 不解密、不读取归属缓存、不修改客户端或已保存的原始历史；原生 bypass 不调用本函数。
func PortableReasoningItems(value any) any {
	items, ok := value.([]any)
	if !ok {
		return value
	}
	out := make([]any, 0, len(items))
	for _, value := range items {
		item, ok := value.(map[string]any)
		if !ok || item["type"] != "reasoning" {
			out = append(out, value)
			continue
		}
		// 网关引用必须交由路由层严格恢复；保留不是授权，未知或过期引用不能被清理成普通历史。
		if strings.HasPrefix(dialogueString(item["encrypted_content"]), "xhub_messages_") {
			out = append(out, value)
			continue
		}
		// 不把错误类型清理成成功请求；原协议校验仍须向调用方报告非法密文。
		if cipher, exists := item["encrypted_content"]; exists && cipher != nil {
			if _, valid := cipher.(string); !valid {
				out = append(out, value)
				continue
			}
		}
		// 参考 LiteLLM _without_encrypted_reasoning：密文不能跨连接复用，来源 ID 也不能证明新连接拥有该推理。
		copy := make(map[string]any, len(item))
		for key, value := range item {
			if key != "encrypted_content" && key != "id" && key != "status" {
				copy[key] = value
			}
		}
		out = append(out, copy)
	}
	return out
}

// PortableResponsesObject 为统一 Responses 的请求或响应创建顶层副本，参数为对象及 input/output 字段名。
// 返回清理账户绑定推理后的独立顶层对象；内部完整历史、其他字段及工具参数不变，无网络或持久化副作用。
// Serve 和原生流的公开输出调用；nil 保持 nil，只有标准 reasoning 被清理，compaction 等未知密文不伪装成公开历史。
func PortableResponsesObject(body map[string]any, field string) map[string]any {
	if body == nil {
		return nil
	}
	out := make(map[string]any, len(body))
	for key, value := range body {
		out[key] = value
	}
	if value, ok := body[field]; ok {
		out[field] = PortableReasoningItems(value)
	}
	return out
}
