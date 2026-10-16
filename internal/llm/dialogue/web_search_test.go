package dialogue

import (
	"encoding/json"
	"errors"
	"testing"
)

// TestPrepareWebSearch 验证可选工具降级、强制搜索拒绝和原生透传；前置纯内存 Responses 请求。
// 正常/边界/失败输入均检查原正文不变；不调用外部服务、不创建持久数据，无需清理。
func TestPrepareWebSearch(t *testing.T) {
	for _, tc := range []struct {
		name, extra                   string
		native, drop, dropped, denied bool
	}{
		{"optional", `"tools":[{"type":"web_search"}]`, false, true, true, false},
		{"preview", `"tools":[{"type":"web_search_preview"}],"tool_choice":"auto"`, false, true, true, false},
		{"default-reject", `"tools":[{"type":"web_search"}]`, false, false, false, true},
		{"native", `"tools":[{"type":"web_search"}],"tool_choice":{"type":"web_search"}`, true, true, false, false},
		{"required", `"tools":[{"type":"web_search"}],"tool_choice":"required"`, false, true, false, true},
		{"forced", `"tools":[{"type":"web_search"}],"tool_choice":{"type":"web_search"}`, false, true, false, true},
		{"forced-without-list", `"tool_choice":{"type":"web_search_preview"}`, false, true, false, true},
		{"allowed-tools-required", `"tools":[{"type":"web_search"}],"tool_choice":{"type":"allowed_tools","mode":"required","tools":[{"type":"web_search"}]}`, false, true, false, true},
		{"only-additional", `"input":[{"type":"additional_tools","tools":[{"type":"web_search"}]},{"role":"user","content":"Hello"}]`, false, true, true, false},
		{"none", `"tools":[{"type":"web_search"}],"tool_choice":"none"`, false, true, true, false},
		{"function-selected", `"tools":[{"type":"web_search"},{"type":"function","name":"read","parameters":{}}],"tool_choice":{"type":"function","name":"read"}`, false, true, true, false},
		{"additional", `"input":[{"type":"additional_tools","tools":[{"type":"web_search"},{"type":"namespace","name":"fs","tools":[{"type":"function","name":"read","parameters":{}}]}]},{"role":"user","content":"Keep this input unchanged."}]`, false, true, true, false},
		{"no-search", `"tools":[{"type":"function","name":"read","parameters":{}}]`, false, true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body map[string]any
			if err := json.Unmarshal([]byte(`{"input":"Keep this input unchanged.",`+tc.extra+`}`), &body); err != nil {
				t.Fatal(err)
			}
			before, _ := json.Marshal(body)
			d, err := ParseDialogue("openai-responses", body)
			if err != nil {
				t.Fatal(err)
			}
			out, dropped, err := PrepareWebSearch(d, tc.native, tc.drop)
			if errors.Is(err, ErrWebSearchUnsupported) != tc.denied || dropped != tc.dropped {
				t.Fatalf("搜索策略结果错误: dropped=%v err=%v", dropped, err)
			}
			after, _ := json.Marshal(body)
			if string(before) != string(after) {
				t.Fatal("候选降级修改了原始请求，可能污染其他部署")
			}
			if err == nil {
				if !tc.native {
					if _, err = EncodeDialogue(out, "openai-chat", "real"); err != nil {
						t.Fatalf("正常工具或文本未保留: %v", err)
					}
				}
				if tc.name == "additional" && len(out.Tools) != 1 {
					t.Fatal("namespace 工具被搜索降级删除")
				}
			}
		})
	}
	// 未知原生工具及字段不能借搜索降级绕过原有能力检查。
	for _, extra := range []string{`"tools":[{"type":"web_search"},{"type":"computer_use_preview"}]`, `"tools":[{"type":"web_search"}],"unknown_capability":true`} {
		var body map[string]any
		_ = json.Unmarshal([]byte(`{"input":"Hello",`+extra+`}`), &body)
		d, err := ParseDialogue("openai-responses", body)
		if err != nil {
			t.Fatal(err)
		}
		out, _, err := PrepareWebSearch(d, false, true)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = EncodeDialogue(out, "openai-chat", "real"); err == nil {
			t.Fatal("搜索降级放行了其他不支持的原生能力")
		}
	}
	if _, dropped, err := PrepareWebSearch(Dialogue{}, false, true); err != nil || dropped {
		t.Fatal("非 Responses 请求被修改")
	}
}
