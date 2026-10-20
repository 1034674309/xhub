package dialogue

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// TestMessagesCitationsProjection 验证缺省/null/空引用可跨协议，真实引用原样重放且禁止有损桥接。
// 前置仅内存 Messages 历史；比较入参不变、原生字段和桥接文本，非法类型返回错误；无需清理外部资源。
func TestMessagesCitationsProjection(t *testing.T) {
	for _, tc := range []struct {
		name, field         string
		invalid, nativeOnly bool
	}{
		{"missing", "", false, false}, {"null", `,"citations":null`, false, false},
		{"empty", `,"citations":[]`, false, false},
		{"real", `,"citations":[{"type":"char_location","cited_text":"Example","document_index":0,"start_char_index":0,"end_char_index":7}]`, false, true},
		{"string", `,"citations":""`, true, false}, {"object", `,"citations":{}`, true, false},
		{"number", `,"citations":0`, true, false}, {"boolean", `,"citations":false`, true, false},
		{"null_item", `,"citations":[null]`, true, false}, {"string_item", `,"citations":["ref"]`, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := `{"model":"public","max_tokens":64,"messages":[{"role":"assistant","content":[{"type":"text","text":"(no content)"` + tc.field + `}]},{"role":"user","content":"Continue."}]}`
			var body map[string]any
			if err := json.Unmarshal([]byte(raw), &body); err != nil {
				t.Fatal(err)
			}
			before, _ := json.Marshal(body)
			d, err := ParseDialogue("anthropic-messages", body)
			if tc.invalid {
				if err == nil {
					t.Fatal("非法引用结构被接受")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			native, err := EncodeDialogue(d, "anthropic-messages", "public")
			if err != nil || !reflect.DeepEqual(native["messages"], body["messages"]) {
				t.Fatalf("原生引用被修改: %v %v", native, err)
			}
			after, _ := json.Marshal(body)
			if string(before) != string(after) {
				t.Fatal("投影修改调用方正文")
			}
			for _, protocol := range []string{"openai-chat", "openai-responses"} {
				mapped, err := EncodeDialogue(d, protocol, "private")
				if tc.nativeOnly {
					if err == nil {
						t.Fatalf("%s 静默丢失真实引用", protocol)
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				encoded, _ := json.Marshal(mapped)
				if !strings.Contains(string(encoded), "(no content)") || strings.Contains(string(encoded), "citations") {
					t.Fatalf("%s 丢失文本: %s", protocol, encoded)
				}
			}
		})
	}
}
