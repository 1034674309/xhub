package dialogue

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// TestNativeChatIndexProjection 验证同步/SDK累积工具索引可原样回传，跨协议只按调用ID关联；错误索引仍拒绝。
// 参数 t 为上下文；纯内存请求不修改原文、不访问外部服务，无需数据清理。
func TestNativeChatIndexProjection(t *testing.T) {
	for _, tc := range []struct {
		name  string
		index any
		ok    bool
	}{{"zero", float64(0), true}, {"sparse", float64(7), true}, {"negative", float64(-1), false}, {"fraction", 1.5, false}, {"string", "0", false}, {"null", nil, false}} {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]any{"messages": []any{map[string]any{"role": "assistant", "content": nil, "tool_calls": []any{map[string]any{"index": tc.index, "id": "call", "type": "function", "function": map[string]any{"name": "lookup", "arguments": "{}"}}}}, map[string]any{"role": "tool", "tool_call_id": "call", "content": "result"}}}
			before, _ := json.Marshal(body)
			d, err := ParseDialogue("openai-chat", body)
			if (err == nil) != tc.ok {
				t.Fatalf("索引契约不符: %v", err)
			}
			if tc.ok {
				native, err := EncodeDialogue(d, "openai-chat", "m")
				if err != nil || !reflect.DeepEqual(native["messages"], body["messages"]) {
					t.Fatalf("原生工具历史变动: %v %v", native, err)
				}
				out, err := EncodeDialogue(d, "openai-responses", "m")
				raw, _ := json.Marshal(out)
				if err != nil || strings.Contains(string(raw), "index") || !strings.Contains(string(raw), "call") {
					t.Fatalf("跨协议索引处理错误: %s %v", raw, err)
				}
			}
			after, _ := json.Marshal(body)
			if string(before) != string(after) {
				t.Fatal("解析修改调用方原文")
			}
		})
	}
}

// TestNativeMessagesCaller 验证直接调用来源同协议保留、跨协议可无损投影，程序化或畸形来源明确拒绝。
// 参数 t 为上下文；本地对象覆盖正常/空值/未知字段，不读写外部数据。
func TestNativeMessagesCaller(t *testing.T) {
	for _, tc := range []struct {
		name   string
		caller any
		ok     bool
	}{{"direct", map[string]any{"type": "direct"}, true}, {"programmatic", map[string]any{"type": "code_execution_20250825"}, false}, {"extra", map[string]any{"type": "direct", "tool_id": "x"}, false}, {"null", nil, false}, {"text", "direct", false}} {
		t.Run(tc.name, func(t *testing.T) {
			block := map[string]any{"type": "tool_use", "id": "c", "name": "read", "input": map[string]any{}, "caller": tc.caller}
			body := map[string]any{"messages": []any{map[string]any{"role": "assistant", "content": []any{block}}}}
			d, err := ParseDialogue("anthropic-messages", body)
			if (err == nil) != tc.ok {
				t.Fatalf("caller契约不符: %v", err)
			}
			if !tc.ok {
				return
			}
			same, err := EncodeDialogue(d, "anthropic-messages", "m")
			if err != nil || !reflect.DeepEqual(same["messages"], body["messages"]) {
				t.Fatalf("caller未原样保留: %v %v", same, err)
			}
			cross, err := EncodeDialogue(d, "openai-chat", "m")
			raw, _ := json.Marshal(cross)
			if err != nil || strings.Contains(string(raw), "caller") || !strings.Contains(string(raw), `"id":"c"`) {
				t.Fatalf("直接调用桥接错误: %s %v", raw, err)
			}
		})
	}
}

// TestNativeMessagesOpaqueRoundTrip 验证原生 thinking/redacted 响应及完整工具历史原样往返，跨协议不泄漏或伪造签名。
// 参数 t 为上下文；JSON/调用ID/签名均为合成值；不访问供应商或数据库，无需清理。
func TestNativeMessagesOpaqueRoundTrip(t *testing.T) {
	for _, kind := range []string{"thinking", "redacted_thinking"} {
		t.Run(kind, func(t *testing.T) {
			block := map[string]any{"type": kind, "thinking": "Plan", "signature": "native-signature"}
			if kind == "redacted_thinking" {
				block = map[string]any{"type": kind, "data": "native-cipher"}
			}
			content := []any{block, map[string]any{"type": "tool_use", "id": "call", "name": "lookup", "input": map[string]any{}, "caller": map[string]any{"type": "direct"}}}
			raw, _ := json.Marshal(map[string]any{"id": "msg", "type": "message", "role": "assistant", "content": content, "stop_reason": "tool_use", "usage": map[string]any{"input_tokens": 8, "output_tokens": 2}})
			r, err := ParseDialogueResult("anthropic-messages", raw)
			if err != nil {
				t.Fatal(err)
			}
			out, err := EncodeDialogueResult(r, "anthropic-messages", "m")
			var same map[string]any
			_ = json.Unmarshal(out, &same)
			if err != nil || !reflect.DeepEqual(same["content"], content) {
				t.Fatalf("原生输出变动: %s %v", out, err)
			}
			d, err := ParseDialogue("anthropic-messages", map[string]any{"messages": []any{map[string]any{"role": "assistant", "content": content}, map[string]any{"role": "user", "content": []any{map[string]any{"type": "tool_result", "tool_use_id": "call", "content": "OK"}}}}})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = EncodeDialogue(d, "anthropic-messages", "m"); err != nil {
				t.Fatal(err)
			}
			for _, p := range []string{"openai-chat", "openai-responses"} {
				if _, err = EncodeDialogueResult(r, p, "m"); err == nil {
					t.Errorf("原生签名被跨协议输出 %s", p)
				}
				if _, err = EncodeDialogue(d, p, "m"); err == nil {
					t.Errorf("原生签名被跨协议重放 %s", p)
				}
			}
		})
	}
}

// TestResponsesParallelHistoryGrouping 验证两轮并行工具各自归入一条 Chat assistant，反序结果仍按原call_id保留。
// 参数 t 为上下文；含文本开场、三个并行调用、工具边界及坏参数；纯内存，不需清理。
func TestResponsesParallelHistoryGrouping(t *testing.T) {
	for _, bad := range []bool{false, true} {
		t.Run(map[bool]string{false: "valid", true: "invalid_arguments"}[bad], func(t *testing.T) {
			items := []any{map[string]any{"role": "user", "content": "Read files."}, map[string]any{"role": "assistant", "content": "Checking."}}
			for _, id := range []string{"a", "b", "c"} {
				args := "{}"
				if bad && id == "c" {
					args = "{"
				}
				items = append(items, map[string]any{"type": "function_call", "call_id": id, "name": "read", "arguments": args})
			}
			for _, id := range []string{"c", "b", "a"} {
				items = append(items, map[string]any{"type": "function_call_output", "call_id": id, "output": id + "-result"})
			}
			items = append(items, map[string]any{"role": "user", "content": "Again."}, map[string]any{"type": "function_call", "call_id": "d", "name": "read", "arguments": "{}"})
			d, err := ParseDialogue("openai-responses", map[string]any{"input": items})
			if bad {
				if err == nil {
					t.Fatal("坏工具参数被合并后放行")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			out, err := EncodeDialogue(d, "openai-chat", "m")
			if err != nil {
				t.Fatal(err)
			}
			messages := out["messages"].([]any)
			if len(messages) != 7 || len(dialogueList(dialogueMap(messages[1])["tool_calls"])) != 3 || len(dialogueList(dialogueMap(messages[6])["tool_calls"])) != 1 {
				t.Fatalf("工具轮次错误分组: %v", messages)
			}
			for i, id := range []string{"c", "b", "a"} {
				if dialogueMap(messages[i+2])["tool_call_id"] != id {
					t.Fatalf("工具结果顺序/ID丢失: %v", messages)
				}
			}
		})
	}
}
