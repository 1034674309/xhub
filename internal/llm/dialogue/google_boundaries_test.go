package dialogue

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestGoogleConfigurationAndPartErrors 验证 Google 配置、文本、函数角色和字段错误在转发之前拒绝。
// 参数 t 为上下文；前置有效文本请求分别替换字段，必须返回可定位错误；纯内存数据无需清理。
func TestGoogleConfigurationAndPartErrors(t *testing.T) {
	for _, tc := range []struct{ name, patch, want string }{
		{"unknown", "{\"extra\":true}", "extra"},
		{"system_field", "{\"systemInstruction\":{\"extra\":true}}", "extra"},
		{"system_parts", "{\"systemInstruction\":{\"parts\":42}}", "array"},
		{"system_part_field", "{\"systemInstruction\":{\"parts\":[{\"extra\":true}]}}", "extra"},
		{"system_text", "{\"systemInstruction\":{\"parts\":[{\"text\":42}]}}", "string"},
		{"content_field", "{\"contents\":[{\"extra\":true}]}", "extra"},
		{"content_role", "{\"contents\":[{\"role\":\"system\",\"parts\":[{\"text\":\"x\"}]}]}", "role"},
		{"content_parts", "{\"contents\":[{\"parts\":42}]}", "parts"},
		{"part_fields", "{\"contents\":[{\"parts\":[{\"text\":\"x\",\"extra\":true}]}]}", "part"},
		{"part_text", "{\"contents\":[{\"parts\":[{\"text\":42}]}]}", "string"},
		{"call_field", "{\"contents\":[{\"role\":\"model\",\"parts\":[{\"functionCall\":{\"extra\":true}}]}]}", "extra"},
		{"call_role", "{\"contents\":[{\"parts\":[{\"functionCall\":{\"name\":\"f\",\"args\":{}}}]}]}", "model role"},
		{"call_args", "{\"contents\":[{\"role\":\"model\",\"parts\":[{\"functionCall\":{\"name\":\"f\",\"args\":[]}}]}]}", "object"},
		{"call_name", "{\"contents\":[{\"role\":\"model\",\"parts\":[{\"functionCall\":{\"args\":{}}}]}]}", "function name"},
		{"response_field", "{\"contents\":[{\"parts\":[{\"functionResponse\":{\"extra\":true}}]}]}", "extra"},
		{"response_role", "{\"contents\":[{\"role\":\"model\",\"parts\":[{\"functionResponse\":{\"name\":\"f\",\"response\":{}}}]}]}", "user role"},
		{"response_object", "{\"contents\":[{\"parts\":[{\"functionResponse\":{\"name\":\"f\",\"response\":[]}}]}]}", "object"},
		{"generation_field", "{\"generationConfig\":{\"extra\":true}}", "extra"},
		{"tools_array", "{\"tools\":42}", "array"},
		{"tools_field", "{\"tools\":[{\"extra\":true}]}", "extra"},
		{"declarations_array", "{\"tools\":[{\"functionDeclarations\":42}]}", "array"},
		{"declaration_field", "{\"tools\":[{\"functionDeclarations\":[{\"extra\":true}]}]}", "extra"},
		{"config_object", "{\"toolConfig\":42}", "object"},
		{"config_field", "{\"toolConfig\":{\"extra\":true}}", "extra"},
		{"function_config_object", "{\"toolConfig\":{\"functionCallingConfig\":42}}", "object"},
		{"function_config_field", "{\"toolConfig\":{\"functionCallingConfig\":{\"extra\":true}}}", "extra"},
		{"function_mode", "{\"toolConfig\":{\"functionCallingConfig\":{\"mode\":\"INVALID\"}}}", "mode"},
		{"function_names", "{\"toolConfig\":{\"functionCallingConfig\":{\"mode\":\"ANY\",\"allowedFunctionNames\":[\"a\",\"b\"]}}}", "one forced"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]any{"contents": []any{map[string]any{"parts": []any{map[string]any{"text": "hello"}}}}}
			var patch map[string]any
			if err := json.Unmarshal([]byte(tc.patch), &patch); err != nil {
				t.Fatal(err)
			}
			for k, v := range patch {
				body[k] = v
			}
			if _, err := ParseDialogue("gemini", body); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Google 未按契约拒绝 %s: %v", tc.name, err)
			}
		})
	}
}

// TestGoogleGeneratedToolIDs 验证原厂省略 ID 时确定性补齐，并把唯一待处理调用关联到结果。
// 参数 t 为上下文；前置一组函数调用/结果，断言参数、结果和 ID 保持；纯内存无需清理。
func TestGoogleGeneratedToolIDs(t *testing.T) {
	raw := "{\"contents\":[{\"role\":\"model\",\"parts\":[{\"functionCall\":{\"name\":\"find\",\"args\":{\"q\":\"x\"}}}]},{\"parts\":[{\"functionResponse\":{\"name\":\"find\",\"response\":{\"found\":true}}}]}]}"
	var body map[string]any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatal(err)
	}
	d, err := ParseDialogue("gemini", body)
	if err != nil || len(d.Turns) != 2 {
		t.Fatalf("Google 工具解析失败: %+v %v", d, err)
	}
	call := d.Turns[0].Calls[0]
	if call.ID != "google_find_0_0" || call.Arguments != "{\"q\":\"x\"}" || d.Turns[1].ID != call.ID || d.Turns[1].Text != "{\"found\":true}" {
		t.Fatalf("工具关联丢失: %+v", d)
	}
}

// TestGoogleResultErrorContract 验证无效原厂 JSON、未知内容、无效计量和缺失函数名不得变成成功回复。
// 参数 t 为上下文；前置各类失败响应，返回明确错误，纯内存无需清理。
func TestGoogleResultErrorContract(t *testing.T) {
	for _, tc := range []struct{ name, raw, want string }{
		{"json", "{", "unexpected"},
		{"sdk_type", "{\"candidates\":42}", "unmarshal"},
		{"upstream", "{\"error\":{\"message\":\"failed\"}}", "upstream error"},
		{"part_null", "{\"candidates\":[{\"content\":{\"parts\":[null]}}]}", "response part"},
		{"unknown_part", "{\"candidates\":[{\"content\":{\"parts\":[{\"inlineData\":{\"mimeType\":\"image/png\",\"data\":\"AA==\"}}]}}]}", "capability"},
		{"missing_name", "{\"candidates\":[{\"content\":{\"parts\":[{\"functionCall\":{\"args\":{}}}]}}]}", "args or name"},
		{"thought_count", "{\"candidates\":[{}],\"usageMetadata\":{\"promptTokenCount\":2,\"candidatesTokenCount\":3,\"thoughtsTokenCount\":-1}}", "thoughts token"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseDialogueResult("gemini", []byte(tc.raw)); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Google 回复失败契约改变: %v", err)
			}
		})
	}
	r, err := ParseDialogueResult("vertex", []byte("{\"candidates\":[{\"content\":{\"parts\":[{\"functionCall\":{\"name\":\"f\",\"args\":{}}}]}}]}"))
	if err != nil || len(r.Calls) != 1 || r.Calls[0].ID != "google_f_0" || r.Stop != "tool_calls" {
		t.Fatalf("省略 ID 的函数回复未补齐: %+v %v", r, err)
	}
}
