package dialogue

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

// TestResultCompletionReasons 验证正常和截断文本回复的五协议往返，包括缺失用量时禁止计价。
// 参数 t 为上下文；前置有效纯文本结果，返回文本/结束原因不改变；无网络与外部清理。
func TestResultCompletionReasons(t *testing.T) {
	for _, protocol := range []string{"openai-chat", "openai-responses", "anthropic-messages", "gemini", "vertex"} {
		for _, stop := range []string{"stop", "length"} {
			t.Run(protocol+"/"+stop, func(t *testing.T) {
				raw, err := EncodeDialogueResult(DialogueResult{ID: "r", Text: "hello", Stop: stop}, protocol, "m")
				if err != nil {
					t.Fatal(err)
				}
				r, err := ParseDialogueResult(protocol, raw)
				if err != nil || r.Text != "hello" || r.Stop != stop || r.Usage["pricing_blocked"] != "upstream_usage_missing" {
					t.Fatalf("文本或截断状态改变: %+v %v", r, err)
				}
				if protocol == "openai-responses" && stop == "length" {
					var m map[string]any
					if err := json.Unmarshal(raw, &m); err != nil {
						t.Fatal(err)
					}
					if dialogueMap(m["incomplete_details"])["reason"] != "max_output_tokens" || m["store"] != false {
						t.Fatalf("截断信息或存储契约错误: %v", m)
					}
				}
			})
		}
	}
}

// TestResultMalformedAndToolErrors 验证通用 JSON、选择数量、工具关联和输出编码的失败行为。
// 参数 t 为上下文；前置恶意或无效原厂响应，错误必须说明原因，无外部数据需清理。
func TestResultMalformedAndToolErrors(t *testing.T) {
	for _, tc := range []struct{ protocol, raw, want string }{
		{"openai-chat", "{", "unexpected"},
		{"openai-chat", "{}", "choices"},
		{"openai-chat", "{\"choices\":[{},{}]}", "choices"},
		{"unknown", "{}", "unregistered"},
		{"openai-chat", "{\"choices\":[{\"message\":{\"tool_calls\":[{\"id\":\"c\",\"type\":\"function\",\"function\":{\"name\":\"f\",\"arguments\":\"broken\"}}]}}]}", "tool call"},
		{"openai-responses", "{\"status\":\"completed\",\"output\":[{\"type\":\"function_call\",\"name\":\"f\",\"arguments\":\"{}\"}]}", "tool call"},
		{"anthropic-messages", "{\"content\":[{\"type\":\"tool_use\",\"id\":\"c\",\"input\":{}}]}", "tool call"},
	} {
		t.Run(tc.protocol+"/"+tc.want, func(t *testing.T) {
			if _, err := ParseDialogueResult(tc.protocol, []byte(tc.raw)); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("错误响应未按契约拒绝: %v", err)
			}
		})
	}
	for _, protocol := range []string{"anthropic-messages", "gemini", "vertex", "unknown"} {
		if _, err := EncodeDialogueResult(DialogueResult{Calls: []Turn{{Arguments: "broken"}}}, protocol, "m"); err == nil {
			t.Fatalf("%s 接受无效输出", protocol)
		}
	}
}

// TestUsageNumericBoundaries 验证整数、JSON 数值、非有限值和非法数值的计量边界；参数 t 为上下文。
// 前置纯内存计量，合法数字解析保持；token 的小数和非法数值明确阻止计价且不修改输入，无需清理。
func TestUsageNumericBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name  string
		v     any
		want  float64
		valid bool
	}{
		{"integer", 2, 2, true}, {"json_number", json.Number("2.5"), 2.5, true}, {"bad_json_number", json.Number("broken"), 0, false},
		{"nan", math.NaN(), 0, false}, {"infinity", math.Inf(1), 0, false}, {"string", "2", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n, valid := dialogueNumber(tc.v)
			if valid != tc.valid || valid && n != tc.want {
				t.Fatalf("数值识别改变: %v %v", n, valid)
			}
			input := map[string]any{"input_tokens": tc.v, "output_tokens": 1}
			out := NormalizeDialogueUsage(input)
			invalidToken := !tc.valid || tc.want != math.Trunc(tc.want)
			if tc.valid && out["total_tokens"] != tc.want+1 || invalidToken && out["pricing_blocked"] != "upstream_usage_invalid" {
				t.Fatalf("无效计量被计价: %v", out)
			}
			if _, ok := input["input_tokens"]; !ok {
				t.Fatal("计量标准化修改了输入")
			}
		})
	}
}
