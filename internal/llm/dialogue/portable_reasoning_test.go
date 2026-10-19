package dialogue

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// TestPortableReasoningItems 验证历史迁移保留可读内容及工具引用，前置含外部密文和普通推理的 JSON。
// 参数 t 为测试上下文，无返回；覆盖空值、非法输入和未知加密类型，不修改源对象，无外部资源需清理。
func TestPortableReasoningItems(t *testing.T) {
	bad := []any{map[string]any{"type": "reasoning", "encrypted_content": float64(42)}}
	if !reflect.DeepEqual(PortableReasoningItems(bad), bad) {
		t.Fatal("非法密文类型被掩盖")
	}
	for _, value := range []any{nil, "Hello", float64(3)} {
		if !reflect.DeepEqual(PortableReasoningItems(value), value) {
			t.Fatalf("非数组输入被掩盖: %v", value)
		}
	}
	source := map[string]any{"input": []any{
		map[string]any{"type": "reasoning", "id": "rs_foreign", "status": "completed", "encrypted_content": "foreign-cipher", "summary": []any{map[string]any{"type": "summary_text", "text": "Plan"}}},
		map[string]any{"type": "reasoning", "id": "rs_empty", "encrypted_content": "hidden", "summary": []any{}},
		map[string]any{"type": "function_call", "id": "fc_1", "call_id": "call_1", "name": "read", "arguments": "{}"},
		map[string]any{"type": "function_call_output", "call_id": "call_1", "output": "file-value"},
		map[string]any{"role": "user", "content": "用户原文"},
		map[string]any{"type": "compaction", "encrypted_content": "must-not-discard"},
		float64(7),
	}}
	original, _ := json.Marshal(source)
	out := PortableResponsesObject(source, "input")
	items := out["input"].([]any)
	for _, index := range []int{0, 1} {
		item := items[index].(map[string]any)
		if item["id"] != nil || item["encrypted_content"] != nil || item["status"] != nil {
			t.Fatalf("迁移推理仍含来源字段: %v", item)
		}
	}
	encoded, _ := json.Marshal(out)
	if !strings.Contains(string(encoded), "Plan") || !strings.Contains(string(encoded), "用户原文") || len(items) != 7 {
		t.Fatalf("可读历史或无效形状被删除: %s", encoded)
	}
	for i := 2; i < len(items); i++ {
		if !reflect.DeepEqual(items[i], source["input"].([]any)[i]) {
			t.Fatalf("非推理项 %d 被修改", i)
		}
	}
	after, _ := json.Marshal(source)
	if string(original) != string(after) {
		t.Fatal("清理污染了客户端原始历史")
	}
	if PortableResponsesObject(nil, "input") != nil {
		t.Fatal("nil 对象被伪造")
	}
	if _, ok := PortableResponsesObject(map[string]any{"model": "test"}, "input")["input"]; ok {
		t.Fatal("缺失 input 被自动补齐")
	}
}
