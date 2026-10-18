package dialogue

import (
	"reflect"
	"strings"
	"testing"
)

// TestResponsesOutputTrackingHistory 验证真实 Fenno 输出追踪字段不会阻断公开历史回放。
// 前置已保存的 assistant 完整项，检查 Responses/Chat/Messages 正文、合法 phase 与 ID 仍保持各自契约；源请求不变，无外部清理。
func TestResponsesOutputTrackingHistory(t *testing.T) {
	item := map[string]any{"type": "message", "id": "msg_native", "role": "assistant", "phase": "final_answer", "status": "completed", "content": []any{map[string]any{"type": "output_text", "text": "Keep the original answer."}}, "internal_chat_message_metadata_passthrough": map[string]any{"create_time": 1792345743.5, "turn_id": "turn-owned-by-client"}, "metadata": map[string]any{"turn_id": "turn-owned-by-client"}}
	body := map[string]any{"input": []any{item, map[string]any{"role": "user", "content": "用户原文"}}}
	original, _ := cloneObject(body)
	d, err := ParseDialogue("openai-responses", body)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"openai-responses", "openai-chat", "anthropic-messages"} {
		out, err := EncodeDialogue(d, target, "real")
		if err != nil {
			t.Fatalf("完整公开追踪历史无法回放到 %s: %v", target, err)
		}
		if target == "openai-responses" {
			native := dialogueMap(dialogueList(out["input"])[0])
			if native["internal_chat_message_metadata_passthrough"] != nil || native["metadata"] != nil || native["phase"] != "final_answer" || native["id"] != "msg_native" {
				t.Fatalf("原生追踪规范化破坏了 message 身份或 phase: %v", native)
			}
		} else {
			if len(dialogueList(out["messages"])) != 2 {
				t.Fatalf("%s 丢失历史消息: %v", target, out)
			}
			if !strings.Contains(dialogueString(dialogueMap(dialogueList(out["messages"])[0])["content"]), "Keep the original answer.") && target == "openai-chat" {
				t.Fatalf("Chat 丢失原文: %v", out)
			}
		}
	}
	if !reflect.DeepEqual(body, original) {
		t.Fatal("追踪字段规范化修改了用户原始请求")
	}
}

// TestResponsesTrackingMetadataBoundaries 验证仅匹配成对追踪字段；额外业务键、不同 turn_id、错误类型、缺字段与非助手消息仍明确拒绝。
// 前置独立内存完整历史，预期正常元数据不被静默删除或伪装为模型能力，无外部资源清理。
func TestResponsesTrackingMetadataBoundaries(t *testing.T) {
	for _, mode := range []string{"extra_business", "mismatch", "bad_time", "bad_metadata", "missing_pair", "user_role"} {
		t.Run(mode, func(t *testing.T) {
			item := map[string]any{"type": "message", "role": "assistant", "content": "Original", "internal_chat_message_metadata_passthrough": map[string]any{"create_time": 1.0, "turn_id": "a"}, "metadata": map[string]any{"turn_id": "a"}}
			switch mode {
			case "extra_business":
				dialogueMap(item["metadata"])["business"] = "must not drop"
			case "mismatch":
				dialogueMap(item["metadata"])["turn_id"] = "b"
			case "bad_time":
				dialogueMap(item["internal_chat_message_metadata_passthrough"])["create_time"] = "yesterday"
			case "bad_metadata":
				item["metadata"] = 42
			case "missing_pair":
				delete(item, "metadata")
			case "user_role":
				item["role"] = "user"
			}
			if _, err := ParseDialogue("openai-responses", map[string]any{"input": []any{item}}); err == nil {
				t.Fatalf("%s 未识别的业务/非法元数据被静默清理", mode)
			}
		})
	}
}
