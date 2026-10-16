package dialogue

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestResponsesMessagesEncryptedRoundTrip 验证有/无公开摘要的密文推理经 Messages 工具续轮完整回到 Responses。
// 参数 t 为测试上下文；前置内存上游结果，断言密文不进入正文、无伪造 ID、Chat 输出扩展保留但 Messages 封装不得转投其他上游；无外部资源需清理。
func TestResponsesMessagesEncryptedRoundTrip(t *testing.T) {
	for _, summary := range []bool{false, true} {
		name := "redacted"
		parts := []any{}
		if summary {
			name = "thinking"
			parts = append(parts, map[string]any{"type": "summary_text", "text": "Read the file."})
		}
		t.Run(name, func(t *testing.T) {
			raw, _ := json.Marshal(map[string]any{"id": "resp_1", "status": "completed", "output": []any{
				map[string]any{"type": "reasoning", "id": "rs_1", "summary": parts, "encrypted_content": "opaque-one"},
				map[string]any{"type": "function_call", "call_id": "call_1", "name": "read", "arguments": "{}"},
			}})
			result, err := ParseDialogueResult("openai-responses", raw)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := EncodeDialogueResult(result, "anthropic-messages", "public")
			if err != nil {
				t.Fatal(err)
			}
			var reply map[string]any
			_ = json.Unmarshal(encoded, &reply)
			content := dialogueList(reply["content"])
			block := dialogueMap(content[0])
			if block["type"] != name && !(name == "redacted" && block["type"] == "redacted_thinking") {
				t.Fatalf("密文推理块类型错误: %s", encoded)
			}
			if summary && block["signature"] != ResponsesReasoningSignaturePrefix+"opaque-one" {
				t.Fatalf("公开摘要未携带不透明封装: %s", encoded)
			}
			if !summary && block["data"] != ResponsesReasoningSignaturePrefix+"opaque-one" {
				t.Fatalf("无摘要密文被丢弃: %s", encoded)
			}
			d, err := ParseDialogue("anthropic-messages", map[string]any{"messages": []any{
				map[string]any{"role": "user", "content": "Read the file."},
				map[string]any{"role": "assistant", "content": content},
				map[string]any{"role": "user", "content": []any{map[string]any{"type": "tool_result", "tool_use_id": "call_1", "content": "用户原文"}}},
			}})
			if err != nil {
				t.Fatal(err)
			}
			back, err := EncodeDialogue(d, "openai-responses", "actual")
			if err != nil {
				t.Fatal(err)
			}
			item := dialogueMap(dialogueList(back["input"])[1])
			if item["encrypted_content"] != "opaque-one" || item["id"] != nil || item["type"] != "reasoning" {
				t.Fatalf("推理密文或省略 ID 的契约错误: %v", back)
			}
			out, _ := json.Marshal(back)
			if !strings.Contains(string(out), "用户原文") || !strings.Contains(string(out), "call_1") {
				t.Fatalf("工具续轮丢失原文或调用身份: %s", out)
			}
			for _, target := range []string{"openai-chat", "anthropic-messages", "gemini"} {
				if _, err := EncodeDialogue(d, target, "other"); err == nil {
					t.Fatalf("Responses 密文被发送到 %s", target)
				}
			}
			if encoded, err := EncodeDialogueResult(result, "openai-chat", "other"); err != nil || !strings.Contains(string(encoded), `"reasoning_items"`) {
				t.Fatalf("Chat 回复未保留 Responses 推理扩展: %v %s", err, encoded)
			}
		})
	}
}

// TestMessagesReasoningEnvelopeBoundaries 验证空封装、字段类型、用户角色、厂商签名及多份推理的分项边界。
// 参数 t 为上下文；前置纯 JSON 数据，正常摘要/空密文保持公开模式，非法输入必须失败且不改原对象；无存储清理。
func TestMessagesReasoningEnvelopeBoundaries(t *testing.T) {
	for _, item := range []map[string]any{
		{"summary": "bad"}, {"summary": []any{map[string]any{"type": "summary_text", "text": 42}}},
		{"summary": []any{}, "encrypted_content": 42},
	} {
		if _, err := MessagesReasoningBlock(item); err == nil {
			t.Fatalf("非法上游推理字段被接受: %v", item)
		}
	}
	block, err := MessagesReasoningBlock(map[string]any{"summary": []any{
		map[string]any{"type": "summary_text", "text": "First."}, map[string]any{"type": "summary_text", "text": "Second."},
	}, "encrypted_content": ""})
	if err != nil || block["thinking"] != "First.\n\nSecond." || block["signature"] != "" {
		t.Fatalf("空密文或多段公开摘要处理错误: %v %v", block, err)
	}
	if block, err := MessagesReasoningBlock(map[string]any{"summary": []any{}}); err != nil || block != nil {
		t.Fatalf("空推理不应产生可见块: %v %v", block, err)
	}
	for _, value := range []any{ResponsesReasoningSignaturePrefix, "vendor-signature", 42, nil} {
		if _, err := messagesReasoningReplay(map[string]any{"type": "redacted_thinking", "data": value}, "assistant"); err == nil {
			t.Fatalf("无效密文封装被接受: %v", value)
		}
	}
	if _, err := messagesReasoningReplay(map[string]any{"type": "thinking", "thinking": "Plan", "signature": ResponsesReasoningSignaturePrefix + "opaque"}, "user"); err == nil {
		t.Fatal("用户内容不得伪装成助手推理")
	}
}
