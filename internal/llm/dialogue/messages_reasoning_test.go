package dialogue

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestMessagesPublicReasoningRoundTrip 验证 Chat/Responses 的公开推理、工具调用和工具结果经 Messages 往返。
// 参数 t 为测试上下文；前置真实协议形状的内存结果，断言推理与正文分离、调用 ID 不变且不伪造签名；无外部数据需清理。
func TestMessagesPublicReasoningRoundTrip(t *testing.T) {
	for protocol, raw := range map[string]string{
		"openai-chat":      `{"id":"r","choices":[{"message":{"role":"assistant","content":null,"reasoning_content":"Read first.","tool_calls":[{"id":"c","type":"function","function":{"name":"read","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`,
		"openai-responses": `{"id":"r","status":"completed","output":[{"id":"rs_r","type":"reasoning","summary":[{"type":"summary_text","text":"Read first."}]},{"type":"function_call","call_id":"c","name":"read","arguments":"{}"}]}`,
	} {
		t.Run(protocol, func(t *testing.T) {
			result, err := ParseDialogueResult(protocol, []byte(raw))
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := EncodeDialogueResult(result, "anthropic-messages", "public")
			if err != nil {
				t.Fatal(err)
			}
			var response map[string]any
			if err := json.Unmarshal(encoded, &response); err != nil {
				t.Fatal(err)
			}
			content := dialogueList(response["content"])
			if len(content) != 2 || dialogueMap(content[0])["thinking"] != "Read first." || dialogueMap(content[1])["id"] != "c" || response["stop_reason"] != "tool_use" {
				t.Fatalf("Messages 未保持推理和工具身份: %s", encoded)
			}
			if signature := dialogueMap(content[0])["signature"]; signature != nil && signature != "" {
				t.Fatal("公开推理不应生成厂商签名")
			}
			body := map[string]any{"messages": []any{
				map[string]any{"role": "user", "content": "Read the file."},
				map[string]any{"role": "assistant", "content": content},
				map[string]any{"role": "user", "content": []any{map[string]any{"type": "tool_result", "tool_use_id": "c", "content": "文件原文"}}},
			}}
			d, err := ParseDialogue("anthropic-messages", body)
			if err != nil {
				t.Fatal(err)
			}
			back, err := EncodeDialogue(d, protocol, "real")
			if err != nil {
				t.Fatal(err)
			}
			bytes, _ := json.Marshal(back)
			if !strings.Contains(string(bytes), "Read first.") || !strings.Contains(string(bytes), "文件原文") || !strings.Contains(string(bytes), `"c"`) {
				t.Fatalf("工具续轮丢失信息: %s", bytes)
			}
			if protocol == "openai-chat" && dialogueMap(dialogueList(back["messages"])[1])["reasoning_content"] != "Read first." {
				t.Fatalf("Chat 推理未还原: %s", bytes)
			}
			if protocol == "openai-responses" && dialogueMap(dialogueList(back["input"])[1])["type"] != "reasoning" {
				t.Fatalf("Responses 推理被误转为普通文本: %s", bytes)
			}
		})
	}
}

// TestMessagesThinkingInputBoundaries 验证公开 thinking 的空值、错误类型、角色、签名与密文边界。
// 参数 t 为测试上下文；纯内存请求允许空/缺失签名，拒绝不能等价桥接的厂商签名和 redacted 块；不修改输入，无需清理。
func TestMessagesThinkingInputBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, role string
		block      map[string]any
		valid      bool
	}{
		{"public", "assistant", map[string]any{"type": "thinking", "thinking": "Plan"}, true},
		{"empty", "assistant", map[string]any{"type": "thinking", "thinking": "", "signature": ""}, true},
		{"null_signature", "assistant", map[string]any{"type": "thinking", "thinking": "Plan", "signature": nil}, true},
		{"wrong_text", "assistant", map[string]any{"type": "thinking", "thinking": 42}, false},
		{"wrong_signature", "assistant", map[string]any{"type": "thinking", "thinking": "Plan", "signature": 42}, false},
		{"signed", "assistant", map[string]any{"type": "thinking", "thinking": "Plan", "signature": "vendor-signature"}, false},
		{"user_thinking", "user", map[string]any{"type": "thinking", "thinking": "Plan"}, false},
		{"redacted", "assistant", map[string]any{"type": "redacted_thinking", "data": "cipher"}, false},
		{"unknown_field", "assistant", map[string]any{"type": "thinking", "thinking": "Plan", "unknown": true}, false},
		{"thinking_on_text", "assistant", map[string]any{"type": "text", "text": "Hello", "thinking": "Plan"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]any{"messages": []any{map[string]any{"role": tc.role, "content": []any{tc.block, map[string]any{"type": "text", "text": "Hello"}}}}}
			before, _ := json.Marshal(body)
			_, err := ParseDialogue("anthropic-messages", body)
			if (err == nil) != tc.valid {
				t.Fatalf("thinking 边界 %s: %v", tc.name, err)
			}
			after, _ := json.Marshal(body)
			if string(before) != string(after) {
				t.Fatal("解析修改了用户原文")
			}
		})
	}
}

// TestPublicReasoningNullableCipher 验证 Codex 补充的 encrypted_content:null 不会阻断公开历史。
// 参数 t 为上下文；同协议保留原文空字段、跨协议只投影公开摘要；纯内存无清理，真正非空密文仍由既有边界测试拒绝。
func TestPublicReasoningNullableCipher(t *testing.T) {
	body := map[string]any{"input": []any{map[string]any{"type": "reasoning", "id": "rs_public", "summary": []any{map[string]any{"type": "summary_text", "text": "Plan"}}, "encrypted_content": nil}, map[string]any{"role": "assistant", "content": "Done"}}}
	d, err := ParseDialogue("openai-responses", body)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"openai-responses", "openai-chat", "anthropic-messages"} {
		out, err := EncodeDialogue(d, target, "real")
		if err != nil {
			t.Fatalf("公开推理空占位阻断 %s: %v", target, err)
		}
		raw, _ := json.Marshal(out)
		if !strings.Contains(string(raw), "Plan") || !strings.Contains(string(raw), "Done") {
			t.Fatalf("公开历史丢失: %s", raw)
		}
		if target == "openai-responses" {
			if _, ok := dialogueMap(dialogueList(out["input"])[0])["encrypted_content"]; !ok {
				t.Fatal("原生空占位被修改")
			}
		}
	}
}
