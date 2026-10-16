package dialogue

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// TestCustomToolInputAlias 验证七牛实测 input 包装与原 content 包装均还原成相同 custom 原文。
// 前置内存映射，覆盖空串、换行、反斜杠、额外字段及错误类型；不执行代码，不访问外部服务，无需清理。
func TestCustomToolInputAlias(t *testing.T) {
	bindings := map[string]ToolBinding{"exec": {Name: "exec", Custom: true}}
	for _, text := range []string{"", "const value = await tools.read({path: \"probe.txt\"});\ntext(value);", `text("\\n");`} {
		for _, field := range []string{"content", "input"} {
			raw, _ := json.Marshal(map[string]any{field: text})
			item, err := ResponseToolItem(Turn{ID: "call", Name: "exec", Arguments: string(raw)}, bindings, "completed")
			if err != nil || item["input"] != text || item["type"] != "custom_tool_call" || item["call_id"] != "call" {
				t.Fatalf("custom %s 单字段包装未无损还原: %v, %v", field, item, err)
			}
		}
	}
	for _, args := range []string{`{"input":42}`, `{"input":"x","content":"y"}`, `{"input":"x","extra":true}`, `null`, `[]`, `{`} {
		if _, err := ResponseToolItem(Turn{Name: "exec", Arguments: args}, bindings, "completed"); err == nil {
			t.Fatalf("歧义或非法 custom 参数应拒绝: %s", args)
		}
	}
}

// TestClientToolSearchRoundTrip 验证客户端工具发现的声明、强制选择、搜索调用/结果及延迟工具加载能双向桥接。
// 前置纯内存 Responses 历史；保留原生正文、用户输入和 call_id，重复定义合并且冲突不覆盖，无外部资源清理。
func TestClientToolSearchRoundTrip(t *testing.T) {
	var body map[string]any
	_ = json.Unmarshal([]byte(`{"input":[{"role":"user","content":"Read the file. 保留原文。"},{"type":"tool_search_call","execution":"client","call_id":"s1","arguments":{"query":"read a file"},"status":"completed"},{"type":"tool_search_output","execution":"client","call_id":"s1","tools":[{"type":"namespace","name":"fs","tools":[{"type":"custom","name":"read","defer_loading":true}]}],"status":"completed"},{"type":"tool_search_output","execution":"client","call_id":"s2","tools":[{"type":"namespace","name":"fs","tools":[{"type":"custom","name":"read","defer_loading":true}]}]}],"tools":[{"type":"tool_search","execution":"client","description":"Discover tools","parameters":{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}}],"tool_choice":{"type":"tool_search"}}`), &body)
	body["store"] = false
	before, _ := json.Marshal(body)
	d, err := ParseDialogue("openai-responses", body)
	if err != nil {
		t.Fatal(err)
	}
	chat, err := EncodeDialogue(d, "openai-chat", "real")
	if err != nil {
		t.Fatal(err)
	}
	tools := dialogueList(chat["tools"])
	if len(tools) != 2 || dialogueMap(dialogueMap(tools[0])["function"])["name"] != "tool_search" || dialogueMap(dialogueMap(tools[1])["function"])["name"] != "fs__read" {
		t.Fatalf("客户端搜索或加载工具丢失/重复: %v", tools)
	}
	raw, _ := json.Marshal(chat)
	if strings.Contains(string(raw), `"defer_loading":`) || !strings.Contains(string(raw), "保留原文") || !strings.Contains(string(raw), `"tool_call_id":"s1"`) || dialogueMap(dialogueMap(chat["tool_choice"])["function"])["name"] != "tool_search" {
		t.Fatalf("搜索历史、选择或原文不正确: %s", raw)
	}
	result := DialogueResult{ID: "r", ToolBindings: d.ToolBindings, Calls: []Turn{{ID: "s3", Name: "tool_search", Arguments: `{"query":"read","limit":2}`}}}
	encoded, err := EncodeDialogueResult(result, "openai-responses", "public")
	if err != nil {
		t.Fatal(err)
	}
	var response map[string]any
	_ = json.Unmarshal(encoded, &response)
	item := dialogueMap(dialogueList(response["output"])[0])
	if item["type"] != "tool_search_call" || item["execution"] != "client" || dialogueMap(item["arguments"])["query"] != "read" || item["call_id"] != "s3" || item["name"] != nil {
		t.Fatalf("搜索调用未恢复客户端协议: %s", encoded)
	}
	native, err := EncodeDialogue(d, "openai-responses", "")
	if err != nil {
		t.Fatal(err)
	}
	delete(native, "model")
	after, _ := json.Marshal(body)
	if !reflect.DeepEqual(native, body) || string(before) != string(after) {
		t.Fatal("客户端搜索投影修改了原生请求")
	}
}

// TestClientToolSearchBoundaries 验证客户端搜索不会误接管托管搜索，错误 schema/历史和工具别名冲突不会静默放行。
// 前置内存请求；成功声明允许桥接，原生托管声明保留但桥接拒绝，不访问网络或数据库，无需清理。
func TestClientToolSearchBoundaries(t *testing.T) {
	for _, execution := range []any{nil, "server", "future", 42} {
		tool := map[string]any{"type": "tool_search", "execution": execution}
		d, err := ParseDialogue("openai-responses", map[string]any{"input": "Hello", "tools": []any{tool}})
		if err == nil {
			_, err = EncodeDialogue(d, "openai-chat", "real")
		}
		if err == nil {
			t.Fatalf("不支持的搜索执行模式未拒绝: %v", execution)
		}
	}
	for _, raw := range []string{
		`{"type":"tool_search_call","execution":"client","call_id":"s","arguments":"bad"}`,
		`{"type":"tool_search_output","execution":"client","call_id":"s","tools":{}}`,
		`{"type":"tool_search_output","execution":"client","tools":[]}`,
		`{"type":"tool_search_call","execution":"server","call_id":"s","arguments":{}}`,
	} {
		var item map[string]any
		_ = json.Unmarshal([]byte(raw), &item)
		d, err := ParseDialogue("openai-responses", map[string]any{"input": []any{map[string]any{"role": "user", "content": "Hello"}, item}})
		if err == nil {
			_, err = EncodeDialogue(d, "openai-chat", "real")
		}
		if err == nil {
			t.Fatalf("不完整或托管搜索历史未拒绝: %s", raw)
		}
	}
	bindings := map[string]ToolBinding{"tool_search": {Name: "tool_search", ClientSearch: true}}
	for _, args := range []string{`null`, `[]`, `"text"`, `{`} {
		if _, err := ResponseToolItem(Turn{Name: "tool_search", Arguments: args}, bindings, "completed"); err == nil {
			t.Fatalf("搜索参数必须是对象: %s", args)
		}
	}
}
