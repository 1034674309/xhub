package dialogue

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// MessagesHistoryPrefix 标识服务端保存的 Messages 历史引用；随机引用不是加密正文，也不构成授权。
// 路由层必须先按调用方、模型、入口和连接校验并解析引用，纯协议层只使用传入的已授权记录。
const MessagesHistoryPrefix = "xhub_messages_v1:"

// MessagesReplay 是经过路由层授权的完整助手消息及其公开 Responses 投影。
// Message 只包含 role/content；Output 不包含最前面的引用 reasoning 项，保持正文与工具的原始顺序。
type MessagesReplay struct {
	Message map[string]any
	Output  []any
}

// IsMessagesHistoryReference 判断值是否声明网关 Messages 引用；仅识别命名空间，不证明归属或存在性。
// 参数为 encrypted_content，返回是否具有保留前缀；解析和 portable 清理调用，空后缀随后必须被拒绝。
func IsMessagesHistoryReference(value any) bool {
	text, ok := value.(string)
	return ok && strings.HasPrefix(text, MessagesHistoryPrefix)
}

// HasOpaqueMessagesResult 判断已校验回复是否含原厂签名或隐藏推理，供执行链决定是否保存引用。
// 参数为原生结果，返回是否需要原连接回放；不解密、不修改、不访问缓存。
func HasOpaqueMessagesResult(result DialogueResult) bool {
	if result.Source != "anthropic-messages" {
		return false
	}
	for _, value := range dialogueList(result.Native["content"]) {
		block := dialogueMap(value)
		if block["type"] == "redacted_thinking" || (block["type"] == "thinking" && dialogueString(block["signature"]) != "") {
			return true
		}
	}
	return false
}

// ResponsesWithMessagesHistory 将原生 Messages 回复及已持久化引用编码为完整 Responses 结果。
// 参数 result 必须经过原生解析，ref 必须由执行链生成并保存；返回含 leading reasoning 引用的结果或能力错误。
// 不保存、解密或伪造签名，引用仅携带缓存标识；完整消息仍由缓存负责保留，公开摘要来自原厂 thinking 文本。
func ResponsesWithMessagesHistory(result DialogueResult, ref string) (DialogueResult, error) {
	if !HasOpaqueMessagesResult(result) {
		return result, nil
	}
	if !IsMessagesHistoryReference(ref) || len(ref) == len(MessagesHistoryPrefix) {
		return DialogueResult{}, fmt.Errorf("Messages history reference is required")
	}
	if result.Refused || result.BridgeError != "opaque Messages thinking requires native Messages" {
		return DialogueResult{}, fmt.Errorf("unsupported Messages history result")
	}
	public := result
	public.Source, public.Native, public.BridgeError, public.Reasoning = "", nil, "", ""
	raw, err := EncodeDialogueResult(public, "openai-responses", dialogueString(result.Native["model"]))
	if err != nil {
		return DialogueResult{}, err
	}
	var response map[string]any
	if err = json.Unmarshal(raw, &response); err != nil {
		return DialogueResult{}, err
	}
	summary := []any{}
	for _, value := range dialogueList(result.Native["content"]) {
		block := dialogueMap(value)
		if block["type"] == "thinking" {
			if text := dialogueString(block["thinking"]); text != "" {
				summary = append(summary, map[string]any{"type": "summary_text", "text": text})
			}
		}
	}
	item := map[string]any{"id": ResponseItemID("rs", result.ID), "type": "reasoning", "summary": summary, "encrypted_content": ref}
	// Responses 把多个文本块合并成一个文本项，但其位置必须与流中首次文本块一致；工具始终保持原始顺序。
	projected := dialogueList(response["output"])
	ordered := []any{item}
	textAdded := false
	for _, value := range dialogueList(result.Native["content"]) {
		block := dialogueMap(value)
		for _, candidate := range projected {
			output := dialogueMap(candidate)
			if block["type"] == "text" && output["type"] == "message" && !textAdded {
				ordered = append(ordered, candidate)
				textAdded = true
			}
			if block["type"] == "tool_use" && output["call_id"] == block["id"] {
				ordered = append(ordered, candidate)
			}
		}
	}
	if len(ordered) != len(projected)+1 {
		return DialogueResult{}, fmt.Errorf("Messages output projection is incomplete")
	}
	response["output"] = ordered
	result.Source, result.Native, result.BridgeError = "openai-responses", response, ""
	return result, nil
}

// ParseResponsesWithMessagesHistory 解析完整 Responses 历史，并把已授权引用及其公开投影恢复为一条原生助手消息。
// body 为护栏处理后的输入，records 只能由路由层提供；缺记录、篡改或不完整组均返回错误，不推测签名。
// Source/Native 保留 Responses 输入以支持 previous_response_id，Turns 中原生消息仅能发往 Messages；不修改调用方对象。
func ParseResponsesWithMessagesHistory(body map[string]any, records map[string]MessagesReplay) (Dialogue, error) {
	items, ok := body["input"].([]any)
	if !ok {
		return ParseDialogue("openai-responses", body)
	}
	original, err := cloneObject(body)
	if err != nil {
		return Dialogue{}, err
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return Dialogue{}, err
	}
	// 占位文本只在本函数私有投影中使用，选取输入正文中不存在的前缀，避免用户文本被误识别为已授权历史。
	marker := "xhub_internal_messages_history_"
	for strings.Contains(string(raw), marker) {
		marker += "_"
	}
	replacements := map[string]map[string]any{}
	projected := []any{}
	for index := 0; index < len(items); index++ {
		item := dialogueMap(items[index])
		if item["type"] != "reasoning" || !IsMessagesHistoryReference(item["encrypted_content"]) {
			projected = append(projected, items[index])
			continue
		}
		ref := dialogueString(item["encrypted_content"])
		record, found := records[ref]
		if !found || len(ref) == len(MessagesHistoryPrefix) {
			return Dialogue{}, fmt.Errorf("Messages history reference is unavailable")
		}
		if record.Message["role"] != "assistant" || !HasOpaqueMessagesResult(DialogueResult{Source: "anthropic-messages", Native: record.Message}) {
			return Dialogue{}, fmt.Errorf("invalid Messages history owner message")
		}
		if index+len(record.Output) >= len(items) {
			return Dialogue{}, fmt.Errorf("Messages history projection is incomplete")
		}
		for j, expected := range record.Output {
			equal, err := equalMessagesProjection(items[index+1+j], expected)
			if err != nil || !equal {
				return Dialogue{}, fmt.Errorf("Messages history projection mismatch at item %d", j)
			}
		}
		native, err := cloneObject(map[string]any{"role": record.Message["role"], "content": record.Message["content"]})
		if err != nil {
			return Dialogue{}, err
		}
		// 再验证记录的协议形状；缓存损坏不能绕过原生内容校验，也不接受缓存中冒充的普通消息。
		check := map[string]any{"model": "history-validation", "messages": []any{native}}
		parsed, err := ParseDialogue("anthropic-messages", check)
		if err != nil || len(parsed.Turns) == 0 {
			return Dialogue{}, fmt.Errorf("invalid cached Messages history")
		}
		placeholder := fmt.Sprintf("%s%d", marker, len(replacements))
		expectedSummary := []any{}
		for _, value := range dialogueList(native["content"]) {
			block := dialogueMap(value)
			if block["type"] == "thinking" && dialogueString(block["thinking"]) != "" {
				expectedSummary = append(expectedSummary, map[string]any{"type": "summary_text", "text": block["thinking"]})
			}
		}
		expectedReference := map[string]any{"type": "reasoning", "summary": expectedSummary, "encrypted_content": ref}
		if equal, err := equalMessagesProjection(item, expectedReference); err != nil || !equal {
			return Dialogue{}, fmt.Errorf("Messages history reasoning summary mismatch")
		}
		replacements[placeholder] = native
		projected = append(projected, map[string]any{"type": "reasoning", "summary": []any{map[string]any{"type": "summary_text", "text": placeholder}}})
		index += len(record.Output)
	}
	if len(replacements) == 0 {
		return ParseDialogue("openai-responses", body)
	}
	copy, _ := cloneObject(body)
	copy["input"] = projected
	result, err := ParseDialogue("openai-responses", copy)
	if err != nil {
		return Dialogue{}, err
	}
	for i := range result.Turns {
		turn := &result.Turns[i]
		if native, ok := replacements[turn.Reasoning]; ok && turn.Role == "reasoning" {
			placeholder := turn.Reasoning
			*turn = Turn{Role: "assistant", NativeProtocol: "anthropic-messages", Native: native}
			delete(replacements, placeholder) // 下方按实际替换数量校验，不依赖客户端提供的 ID。
		}
	}
	if len(replacements) != 0 {
		return Dialogue{}, fmt.Errorf("Messages history restoration incomplete")
	}
	result.Native = original
	return result, nil
}

// equalMessagesProjection 比较引用后公开项的业务语义，忽略合法 SDK 元数据并规范 JSON 参数空白。
// 参数为实际项和缓存项，返回相等与格式错误；恢复器调用，不修改输入，call_id、类型、正文和工具名始终参与比较。
func equalMessagesProjection(actual, expected any) (bool, error) {
	a, err := normalizeMessagesProjection(actual)
	if err != nil {
		return false, err
	}
	b, err := normalizeMessagesProjection(expected)
	if err != nil {
		return false, err
	}
	return reflect.DeepEqual(a, b), nil
}

// normalizeMessagesProjection 创建用于完整性比较的独立项，参数为 Responses 输出项，返回规范副本或字段错误。
// 只忽略明确的展示元数据，未知字段仍参与相等比较；函数参数必须是合法 JSON，不转换任意字符串或隐藏工具身份。
func normalizeMessagesProjection(value any) (map[string]any, error) {
	item, err := cloneObject(dialogueMap(value))
	if err != nil {
		return nil, err
	}
	if v := item["id"]; v != nil {
		if _, ok := v.(string); !ok {
			return nil, fmt.Errorf("invalid history item id")
		}
	}
	delete(item, "id")
	if v := item["status"]; v != nil && v != "completed" {
		return nil, fmt.Errorf("invalid history item status")
	}
	delete(item, "status")
	if item["type"] == "message" {
		if v := item["phase"]; v != nil && v != "commentary" && v != "final_answer" {
			return nil, fmt.Errorf("invalid history message phase")
		}
		delete(item, "phase")
		for _, value := range dialogueList(item["content"]) {
			part := dialogueMap(value)
			if annotations, ok := part["annotations"].([]any); ok && len(annotations) == 0 {
				delete(part, "annotations")
			}
			if part["annotations"] == nil {
				delete(part, "annotations")
			}
		}
	}
	if item["type"] == "function_call" {
		var arguments any
		text, ok := item["arguments"].(string)
		if !ok || json.Unmarshal([]byte(text), &arguments) != nil {
			return nil, fmt.Errorf("invalid history tool arguments")
		}
		item["arguments"] = arguments
	}
	return item, nil
}
