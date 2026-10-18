package dialogue

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// TestResponseItemID 验证合成 ID 的类型、稳定性和特殊上游标识边界；纯内存输入，无外部依赖或清理。
func TestResponseItemID(t *testing.T) {
	for _, seed := range []string{"chatcmpl-example", "msg_native", "", "invalid /\n中文", strings.Repeat("x", 10000)} {
		for _, prefix := range []string{"msg", "rs", "fc"} {
			id := ResponseItemID(prefix, seed)
			if !regexp.MustCompile("^"+prefix+"_xhub_[0-9a-f]{32}$").MatchString(id) || id != ResponseItemID(prefix, seed) {
				t.Fatalf("合成 %s ID 不合法或不稳定: %q", prefix, id)
			}
		}
		if ResponseItemID("msg", seed) == ResponseItemID("rs", seed) || ResponseItemID("msg", seed) == ResponseItemID("msg", seed+"next") {
			t.Fatal("不同类型或响应的 ID 发生碰撞")
		}
	}
}

// TestResponsesGeneratedTextItemIDs 验证同步 Chat 桥接生成 msg_/rs_，原生 Responses ID 与工具关联不变；内存编码，无需清理。
func TestResponsesGeneratedTextItemIDs(t *testing.T) {
	r := DialogueResult{ID: "chatcmpl-example", Text: "OK", Reasoning: "Plan", Calls: []Turn{{ID: "call_keep", Name: "read", Arguments: "{}"}}, Usage: map[string]any{"prompt_tokens": 8, "completion_tokens": 2}}
	raw, err := EncodeDialogueResult(r, "openai-responses", "public")
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	items := dialogueList(out["output"])
	if dialogueMap(items[0])["id"] != ResponseItemID("rs", r.ID) || dialogueMap(items[1])["id"] != ResponseItemID("msg", r.ID) || dialogueMap(items[2])["call_id"] != "call_keep" || out["id"] != r.ID {
		t.Fatalf("同步桥接混淆 item/response/tool ID: %s", raw)
	}
	native := map[string]any{"id": "resp_native", "output": []any{map[string]any{"type": "message", "id": "msg_native", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": "OK"}}}}}
	r.Source, r.Native = "openai-responses", native
	raw, err = EncodeDialogueResult(r, "openai-responses", "public")
	if err != nil || !strings.Contains(string(raw), `"id":"msg_native"`) {
		t.Fatalf("原生消息 ID 被改写: %v %s", err, raw)
	}
}

// TestLegacyResponsesTextHistoryIDs 验证旧完整公开历史修复及原生/密文/引用保护；原始请求不变，非法输入仍失败，无外部清理。
func TestLegacyResponsesTextHistoryIDs(t *testing.T) {
	for _, tc := range []struct {
		name, kind, id    string
		opaque, reference bool
		want              string
	}{
		{name: "legacy_message", kind: "message", id: "chatcmpl-old_message", want: ResponseItemID("msg", "chatcmpl-old")},
		{name: "legacy_reasoning", kind: "reasoning", id: "chatcmpl-old_reasoning", want: ""},
		{name: "synthetic_reasoning", kind: "reasoning", id: ResponseItemID("rs", "current"), want: ""},
		{name: "native_reasoning", kind: "reasoning", id: "rs_native", want: "rs_native"},
		{name: "native_message", kind: "message", id: "msg_native_message", want: "msg_native_message"},
		{name: "other_invalid_id", kind: "message", id: "foreign_bad", want: "foreign_bad"},
		{name: "opaque_reasoning", kind: "reasoning", id: "chatcmpl-old_reasoning", opaque: true, want: "chatcmpl-old_reasoning"},
		{name: "reference", kind: "message", id: "chatcmpl-old_message", reference: true, want: "chatcmpl-old_message"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := map[string]any{"type": tc.kind, "id": tc.id}
			if tc.kind == "message" {
				item["role"], item["content"] = "assistant", []any{map[string]any{"type": "output_text", "text": "Preserve my text."}}
			} else {
				item["summary"] = []any{map[string]any{"type": "summary_text", "text": "Plan"}}
			}
			if tc.opaque {
				item["encrypted_content"] = "opaque-keep"
			}
			input := []any{map[string]any{"role": "user", "content": "Hello"}, item}
			if tc.reference {
				input = append(input, map[string]any{"type": "item_reference", "id": tc.id})
			}
			body := map[string]any{"input": input}
			original, _ := cloneObject(body)
			d, err := ParseDialogue("openai-responses", body)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := EncodeDialogue(d, "openai-responses", "real")
			if err != nil {
				t.Fatal(err)
			}
			actual := dialogueMap(dialogueList(encoded["input"])[1])
			if (tc.want == "" && actual["id"] != nil) || (tc.want != "" && actual["id"] != tc.want) || !reflect.DeepEqual(body, original) {
				t.Fatalf("旧历史修复或输入隔离错误: %v", encoded)
			}
			if tc.opaque && actual["encrypted_content"] != "opaque-keep" {
				t.Fatal("修复 ID 改变了密文")
			}
		})
	}
	for _, input := range []any{42, []any{42}, []any{map[string]any{"role": "assistant", "content": 42}}} {
		if _, err := ParseDialogue("openai-responses", map[string]any{"input": input}); err == nil {
			t.Fatalf("错误历史输入被接受: %v", input)
		}
	}
}

// TestGrokPublicHistoryIDs 验证真实 Grok UUID 旧桥接与原生公开摘要的跨厂商回放映射。
// 参数由表驱动提供旧 ID、公开状态、密文和引用边界；应只规范已识别完整公开项，正文与源请求不变，无外部数据清理。
func TestGrokPublicHistoryIDs(t *testing.T) {
	const seed = "92ae29c5-9e2c-9cbd-a94b-2fd2abc135be"
	for _, tc := range []struct {
		name, kind, id, wantID string
		status                 any
		opaque, reference      bool
		wantStatus             any
	}{
		{name: "legacy_message", kind: "message", id: seed + "_message", wantID: ResponseItemID("msg", seed)},
		{name: "legacy_reasoning", kind: "reasoning", id: seed + "_reasoning"},
		{name: "native_public_reasoning", kind: "reasoning", id: "rs_" + seed, status: "completed"},
		{name: "native_public_null_status", kind: "reasoning", id: "rs_" + seed},
		{name: "opaque_reasoning", kind: "reasoning", id: "rs_" + seed, status: "completed", opaque: true, wantID: "rs_" + seed, wantStatus: "completed"},
		{name: "referenced_reasoning", kind: "reasoning", id: "rs_" + seed, status: "completed", reference: true, wantID: "rs_" + seed, wantStatus: "completed"},
		{name: "other_native_reasoning", kind: "reasoning", id: "rs_native", status: "completed", wantID: "rs_native", wantStatus: "completed"},
		{name: "invalid_status", kind: "reasoning", id: "rs_" + seed, status: "invalid", wantID: "rs_" + seed, wantStatus: "invalid"},
		{name: "unrecognized_message_suffix", kind: "message", id: "foreign_message", wantID: "foreign_message"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := map[string]any{"type": tc.kind, "id": tc.id}
			if tc.kind == "message" {
				item["role"], item["content"] = "assistant", []any{map[string]any{"type": "output_text", "text": "Old answer"}}
			} else {
				item["summary"], item["status"] = []any{map[string]any{"type": "summary_text", "text": "Keep this public summary."}}, tc.status
			}
			if tc.opaque {
				item["encrypted_content"] = "cipher-keep"
			}
			input := []any{item, map[string]any{"role": "user", "content": "Continue"}}
			if tc.reference {
				input = append(input, map[string]any{"type": "item_reference", "id": tc.id})
			}
			body := map[string]any{"input": input}
			original, _ := cloneObject(body)
			d, err := ParseDialogue("openai-responses", body)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := EncodeDialogue(d, "openai-responses", "native")
			if err != nil {
				t.Fatal(err)
			}
			actual := dialogueMap(dialogueList(encoded["input"])[0])
			_, statusPresent := actual["status"]
			if dialogueString(actual["id"]) != tc.wantID || actual["status"] != tc.wantStatus || (tc.wantStatus == nil && statusPresent) || !reflect.DeepEqual(body, original) {
				t.Fatalf("Grok 历史 ID/status 或请求隔离错误: %v", actual)
			}
			if tc.kind == "reasoning" && !reflect.DeepEqual(actual["summary"], item["summary"]) {
				t.Fatal("公开摘要内容被改写")
			}
			if tc.opaque && actual["encrypted_content"] != "cipher-keep" {
				t.Fatal("Grok 密文被改写")
			}
		})
	}
}

// TestResponsesFunctionItemIDs 验证函数项与关联 call_id 分离、旧完整函数历史修复及原生/引用边界。
// 前置内存同步回复与工具结果；新旧回放保持参数和工具关联，原生 fc_ 与引用不改，无外部清理。
func TestResponsesFunctionItemIDs(t *testing.T) {
	call := Turn{ID: "call-old", Name: "lookup_value", Arguments: "{}"}
	item, err := ResponseToolItem(call, nil, "completed")
	if err != nil || item["id"] != ResponseItemID("fc", call.ID) || item["call_id"] != call.ID {
		t.Fatalf("函数 item/call 身份混淆: %v %v", item, err)
	}
	for _, tc := range []struct {
		name, id  string
		reference bool
		want      string
	}{
		{name: "legacy", id: call.ID, want: ResponseItemID("fc", call.ID)},
		{name: "native", id: "fc_native", want: "fc_native"},
		{name: "arbitrary", id: "foreign_invalid", want: "foreign_invalid"},
		{name: "reference", id: call.ID, reference: true, want: call.ID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			history := []any{map[string]any{"type": "function_call", "id": tc.id, "call_id": call.ID, "name": call.Name, "arguments": call.Arguments}, map[string]any{"type": "function_call_output", "call_id": call.ID, "output": "KNOWN_TEST_VALUE"}}
			if tc.reference {
				history = append(history, map[string]any{"type": "item_reference", "id": tc.id})
			}
			d, err := ParseDialogue("openai-responses", map[string]any{"input": history})
			if err != nil {
				t.Fatal(err)
			}
			native, err := EncodeDialogue(d, "openai-responses", "real")
			if err != nil {
				t.Fatal(err)
			}
			got := dialogueMap(dialogueList(native["input"])[0])
			if got["id"] != tc.want || got["call_id"] != call.ID || got["arguments"] != "{}" {
				t.Fatalf("函数历史规范化丢失工具关联: %v", got)
			}
		})
	}
}
