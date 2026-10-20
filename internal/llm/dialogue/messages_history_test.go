package dialogue

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// signedHistoryFixture 创建签名、隐藏推理和并行工具均存在的原生回复；参数 t 报告夹具错误，返回私有结果，无外部数据无需清理。
func signedHistoryFixture(t *testing.T) DialogueResult {
	t.Helper()
	raw := `{"id":"msg_signed","type":"message","role":"assistant","model":"claude","content":[{"type":"thinking","thinking":"Public plan","signature":"SECRET_SIGNATURE"},{"type":"redacted_thinking","data":"SECRET_REDACTED"},{"type":"tool_use","id":"c1","name":"read","input":{"path":"a"}},{"type":"text","text":"Reading"},{"type":"tool_use","id":"c2","name":"read","input":{"path":"b"}}],"stop_reason":"tool_use","usage":{"input_tokens":10,"output_tokens":12}}`
	result, err := ParseDialogueResult("anthropic-messages", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// signedHistoryRequest 为引用回放构造首轮用户、完整助手投影和两个工具结果；返回独立正文，无网络副作用。
func signedHistoryRequest(t *testing.T, result DialogueResult) map[string]any {
	t.Helper()
	input := []any{map[string]any{"role": "user", "content": "Read both files"}}
	input = append(input, dialogueList(result.Native["output"])...)
	input = append(input, map[string]any{"type": "function_call_output", "call_id": "c1", "output": "A"}, map[string]any{"type": "function_call_output", "call_id": "c2", "output": "B"})
	body, err := cloneObject(map[string]any{"model": "claude", "input": input})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

// TestMessagesHistoryReferenceRoundTrip 验证缓存引用不泄露签名，完整回放只产生一个助手消息并保留原始块顺序。
// 前置为签名、redacted、工具先于文字及并行工具；检查元数据和 JSON 空白容忍、原生协议限定及原输入不变，无外部清理。
func TestMessagesHistoryReferenceRoundTrip(t *testing.T) {
	original := signedHistoryFixture(t)
	ref := MessagesHistoryPrefix + strings.Repeat("a", 64)
	prepared, err := ResponsesWithMessagesHistory(original, ref)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(prepared.Native)
	if strings.Contains(string(raw), "SECRET_") {
		t.Fatalf("签名泄露到Responses: %s", raw)
	}
	output := dialogueList(prepared.Native["output"])
	if len(output) != 4 || dialogueMap(output[1])["call_id"] != "c1" || dialogueMap(output[2])["type"] != "message" {
		t.Fatalf("公开输出顺序错误: %s", raw)
	}
	record := MessagesReplay{Message: original.Native, Output: output[1:]}
	body := signedHistoryRequest(t, prepared)
	items := dialogueList(body["input"])
	dialogueMap(items[2])["arguments"] = `{ "path" : "a" }`
	dialogueMap(items[3])["phase"] = "commentary"
	delete(dialogueMap(items[3]), "id")
	before, _ := json.Marshal(body)
	parsed, err := ParseResponsesWithMessagesHistory(body, map[string]MessagesReplay{ref: record})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeDialogue(parsed, "anthropic-messages", "real-claude")
	if err != nil {
		t.Fatal(err)
	}
	messages := dialogueList(encoded["messages"])
	if len(messages) != 3 || !reflect.DeepEqual(dialogueMap(messages[1])["content"], original.Native["content"]) {
		t.Fatalf("原生助手/工具分组损坏: %#v", messages)
	}
	if len(dialogueList(dialogueMap(messages[2])["content"])) != 2 {
		t.Fatal("并行工具结果未合并")
	}
	if parsed.Source != "openai-responses" || !reflect.DeepEqual(parsed.Native, body) {
		t.Fatal("完整Responses历史未保留")
	}
	if _, err = EncodeDialogue(parsed, "openai-chat", "gpt"); err == nil {
		t.Fatal("原厂签名泄露到非Messages协议")
	}
	after, _ := json.Marshal(body)
	if string(before) != string(after) {
		t.Fatal("回放修改调用方正文")
	}
	if !reflect.DeepEqual(PortableResponsesObject(prepared.Native, "output"), prepared.Native) {
		t.Fatal("portable过滤删除受控引用")
	}
}

// TestMessagesHistoryRejectsTamperedGroups 验证丢失记录、错序/篡改工具、缺项、修改摘要和非法元数据均明确拒绝。
// 前置为完整缓存记录，逐例修改独立请求；要求恢复器返回错误且不外发，无外部数据清理。
func TestMessagesHistoryRejectsTamperedGroups(t *testing.T) {
	original := signedHistoryFixture(t)
	ref := MessagesHistoryPrefix + strings.Repeat("b", 64)
	prepared, err := ResponsesWithMessagesHistory(original, ref)
	if err != nil {
		t.Fatal(err)
	}
	record := MessagesReplay{Message: original.Native, Output: dialogueList(prepared.Native["output"])[1:]}
	cases := map[string]func(map[string]any){
		"missing_record": func(body map[string]any) {
			dialogueMap(dialogueList(body["input"])[1])["encrypted_content"] = MessagesHistoryPrefix + "missing"
		},
		"modified_summary": func(body map[string]any) { dialogueMap(dialogueList(body["input"])[1])["summary"] = []any{} },
		"tool_identity":    func(body map[string]any) { dialogueMap(dialogueList(body["input"])[2])["call_id"] = "other" },
		"tool_argument": func(body map[string]any) {
			dialogueMap(dialogueList(body["input"])[2])["arguments"] = `{"path":"secret"}`
		},
		"order": func(body map[string]any) {
			items := dialogueList(body["input"])
			items[2], items[3] = items[3], items[2]
		},
		"missing_item":     func(body map[string]any) { body["input"] = dialogueList(body["input"])[:3] },
		"unexpected_field": func(body map[string]any) { dialogueMap(dialogueList(body["input"])[2])["unexpected"] = true },
		"wrong_status":     func(body map[string]any) { dialogueMap(dialogueList(body["input"])[2])["status"] = "in_progress" },
		"wrong_id_type":    func(body map[string]any) { dialogueMap(dialogueList(body["input"])[2])["id"] = 42 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			body := signedHistoryRequest(t, prepared)
			mutate(body)
			if _, err := ParseResponsesWithMessagesHistory(body, map[string]MessagesReplay{ref: record}); err == nil {
				t.Fatal("篡改历史被接受")
			}
		})
	}
}

// TestMessagesHistoryPreparationBoundaries 验证普通文本保持原样、无引用签名拒绝、坏缓存拒绝及未知版本保留给路由层。
// 前置为纯内存结果和请求，检查所有边界无需外部服务，无需清理。
func TestMessagesHistoryPreparationBoundaries(t *testing.T) {
	original := signedHistoryFixture(t)
	for _, ref := range []string{"", MessagesHistoryPrefix, "foreign"} {
		if _, err := ResponsesWithMessagesHistory(original, ref); err == nil {
			t.Fatalf("非法引用被接受: %q", ref)
		}
	}
	normal := DialogueResult{ID: "normal", Text: "Hello"}
	if got, err := ResponsesWithMessagesHistory(normal, ""); err != nil || !reflect.DeepEqual(got, normal) {
		t.Fatal("普通文本被改写")
	}
	body := map[string]any{"input": "Hello", "model": "test"}
	if _, err := ParseResponsesWithMessagesHistory(body, nil); err != nil {
		t.Fatal(err)
	}
	future := map[string]any{"type": "reasoning", "summary": []any{}, "encrypted_content": "xhub_messages_v2:unsupported"}
	if !reflect.DeepEqual(PortableReasoningItems([]any{future}), []any{future}) {
		t.Fatal("未知网关引用被静默移除")
	}
	ref := MessagesHistoryPrefix + strings.Repeat("c", 64)
	prepared, _ := ResponsesWithMessagesHistory(original, ref)
	bad := MessagesReplay{Message: map[string]any{"role": "user", "content": []any{}}, Output: dialogueList(prepared.Native["output"])[1:]}
	if _, err := ParseResponsesWithMessagesHistory(signedHistoryRequest(t, prepared), map[string]MessagesReplay{ref: bad}); err == nil {
		t.Fatal("损坏缓存被接受")
	}
}

// TestMessagesHistoryRedactedOnly 验证没有可见推理、文字或工具的隐藏推理也生成唯一引用并可恢复。
// 前置为已验证原生 redacted 块，检查不伪造公开摘要，纯内存执行无需清理。
func TestMessagesHistoryRedactedOnly(t *testing.T) {
	original, err := ParseDialogueResult("anthropic-messages", []byte(`{"id":"r","type":"message","role":"assistant","model":"claude","content":[{"type":"redacted_thinking","data":"SECRET"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`))
	if err != nil {
		t.Fatal(err)
	}
	ref := MessagesHistoryPrefix + strings.Repeat("d", 64)
	prepared, err := ResponsesWithMessagesHistory(original, ref)
	if err != nil {
		t.Fatal(err)
	}
	output := dialogueList(prepared.Native["output"])
	if len(output) != 1 || len(dialogueList(dialogueMap(output[0])["summary"])) != 0 {
		t.Fatal("隐藏推理变成公开文本")
	}
	body := map[string]any{"model": "claude", "input": append([]any{map[string]any{"role": "user", "content": "Think"}}, output...)}
	parsed, err := ParseResponsesWithMessagesHistory(body, map[string]MessagesReplay{ref: {Message: original.Native, Output: []any{}}})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeDialogue(parsed, "anthropic-messages", "claude")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(dialogueMap(dialogueList(encoded["messages"])[1])["content"], original.Native["content"]) {
		t.Fatal("隐藏推理未原样恢复")
	}
}

// TestMessagesHistoryLargeToolInteger 验证完整性比较不因浮点舍入把不同大整数工具参数视为相同。
// 前置为相邻且超出 float64 精确范围的 JSON 整数，检查拒绝篡改并接受仅空白差异，无外部数据清理。
func TestMessagesHistoryLargeToolInteger(t *testing.T) {
	expected := map[string]any{"type": "function_call", "call_id": "c", "name": "read", "arguments": `{"n":9007199254740992}`}
	changed := map[string]any{"type": "function_call", "call_id": "c", "name": "read", "arguments": `{"n":9007199254740993}`}
	if equal, err := equalMessagesProjection(changed, expected); err != nil || equal {
		t.Fatalf("不同大整数被错误接受: equal=%v err=%v", equal, err)
	}
	changed["arguments"] = `{ "n": 9007199254740992 }`
	if equal, err := equalMessagesProjection(changed, expected); err != nil || !equal {
		t.Fatalf("参数空白差异被错误拒绝: equal=%v err=%v", equal, err)
	}
}
