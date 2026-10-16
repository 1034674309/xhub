package dialogue

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	llmdialogue "github.com/sunqirui1987/xhub/internal/llm/dialogue"
)

// TestClientSearchAndCustomInputStream 验证真实 Chat SSE 的客户端搜索/七牛 custom 别名还原和失败终态。
// 参数 t 为上下文；前置碎片化参数流，断言搜索对象仅发 added/done、custom 原文事件及非法参数不完成，无外部资源。
func TestClientSearchAndCustomInputStream(t *testing.T) {
	for _, tc := range []struct {
		name, args, kind string
		binding          llmdialogue.ToolBinding
		valid            bool
	}{
		{"tool_search", `{"query":"read"}`, "tool_search_call", llmdialogue.ToolBinding{Name: "tool_search", ClientSearch: true}, true},
		{"exec", `{"input":"text(1);"}`, "custom_tool_call", llmdialogue.ToolBinding{Name: "exec", Custom: true}, true},
		{"tool_search", `[]`, "tool_search_call", llmdialogue.ToolBinding{Name: "tool_search", ClientSearch: true}, false},
	} {
		t.Run(tc.name+tc.args, func(t *testing.T) {
			raw := fmt.Sprintf("data: {\"id\":\"r\",\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":%q,\"arguments\":%q}}]},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n", tc.name, tc.args)
			response := &http.Response{Body: io.NopCloser(strings.NewReader(raw))}
			_, _, _, output, err := PipeDialogueWithTools(httptest.NewRecorder(), response, time.Now(), "public", "openai-chat", "openai-responses", map[string]llmdialogue.ToolBinding{tc.name: tc.binding})
			if !tc.valid {
				if err == nil || strings.Contains(string(output), "response.completed") {
					t.Fatalf("错误搜索参数不应完成: %v %s", err, output)
				}
				return
			}
			if err != nil || !strings.Contains(string(output), tc.kind) || !strings.Contains(string(output), "response.output_item.done") || !strings.Contains(string(output), "response.completed") {
				t.Fatalf("工具事件未完整还原: %v %s", err, output)
			}
			if tc.binding.ClientSearch && (strings.Contains(string(output), "response.function_call_arguments") || !strings.Contains(string(output), `"arguments":{"query":"read"}`)) {
				t.Fatalf("搜索对象被误编码为函数字符串增量: %s", output)
			}
		})
	}
}
