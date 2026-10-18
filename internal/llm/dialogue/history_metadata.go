package dialogue

// normalizeResponsesMessageMetadata 移除已识别的供应商输出追踪字段，供完整公开历史再次作为 Responses 输入时使用。
// 参数 body 为 ParseDialogue 已深拷贝的正文，原地规范 assistant message，无返回值、无外部副作用。
// Fenno 偶发输出 create_time/turn_id 及同值 metadata.turn_id；它们不参与推理，但会使共享解析及严格原生上游拒绝整段历史。
// 只识别成对、同 turn_id 且没有额外业务键的追踪对象；未知字段/类型仍交既有验证拒绝，正文、phase、原生 ID 与工具关联不改动。
func normalizeResponsesMessageMetadata(body map[string]any) {
	for _, value := range dialogueList(body["input"]) {
		item := dialogueMap(value)
		if item["type"] != "message" || item["role"] != "assistant" {
			continue
		}
		trace, metadata := dialogueMap(item["internal_chat_message_metadata_passthrough"]), dialogueMap(item["metadata"])
		if len(trace) != 2 || len(metadata) != 1 {
			continue
		}
		if _, ok := dialogueNumber(trace["create_time"]); !ok {
			continue
		}
		turnID := dialogueString(trace["turn_id"])
		if turnID == "" || metadata["turn_id"] != turnID {
			continue
		}
		delete(item, "internal_chat_message_metadata_passthrough")
		delete(item, "metadata")
	}
}
