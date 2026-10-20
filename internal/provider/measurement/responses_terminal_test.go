package measurement

import (
	"encoding/json"
	"testing"

	llmdialogue "github.com/sunqirui1987/xhub/internal/llm/dialogue"
)

// lengthResponse 构造自包含的 Responses 长度终态；测试可独立修改返回对象，纯内存无需清理。
func lengthResponse() map[string]any {
	return map[string]any{"id": "resp_limit", "object": "response", "status": "incomplete", "incomplete_details": map[string]any{"reason": "max_output_tokens"}, "output": []any{map[string]any{"type": "message", "id": "msg_limit", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": "LIMIT_OK"}}}}, "usage": map[string]any{"input_tokens": 8, "output_tokens": 8}}
}

// responsesEvent 序列化独立终态夹具；参数为事件类型和快照，返回原始 SSE，不执行外部调用。
func responsesEvent(event string, response map[string]any) []byte {
	raw, _ := json.Marshal(map[string]any{"type": event, "response": response})
	return []byte("event: " + event + "\ndata: " + string(raw) + "\n\n")
}

// TestNativeResponsesLengthTerminal 验证明确输出上限允许结算且原状态不变；其他截断原因和坏输出保留失败。
// 前置本地快照，逐例同时覆盖 JSON 用量及 SSE 结束状态；不连接服务，不产生数据。
func TestNativeResponsesLengthTerminal(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(map[string]any)
		valid  bool
	}{
		{"text", func(map[string]any) {}, true},
		{"empty-output", func(d map[string]any) { d["output"] = []any{} }, true},
		{"complete-tool", func(d map[string]any) {
			d["output"] = []any{map[string]any{"type": "function_call", "id": "fc_limit", "call_id": "call_limit", "name": "lookup", "arguments": "{}"}}
		}, true},
		{"truncated-tool", func(d map[string]any) {
			d["output"] = []any{map[string]any{"type": "function_call", "call_id": "call_limit", "name": "lookup", "arguments": "{"}}
		}, true},
		{"numeric-tool-arguments", func(d map[string]any) {
			d["output"] = []any{map[string]any{"type": "function_call", "call_id": "call_limit", "name": "lookup", "arguments": 42}}
		}, false},
		{"object-tool-arguments", func(d map[string]any) {
			d["output"] = []any{map[string]any{"type": "function_call", "call_id": "call_limit", "name": "lookup", "arguments": map[string]any{}}}
		}, false},
		{"missing-tool-arguments", func(d map[string]any) {
			d["output"] = []any{map[string]any{"type": "function_call", "call_id": "call_limit", "name": "lookup"}}
		}, false},
		{"unknown-reason", func(d map[string]any) { d["incomplete_details"] = map[string]any{"reason": "other"} }, false},
		{"content-filter", func(d map[string]any) { d["incomplete_details"] = map[string]any{"reason": "content_filter"} }, false},
		{"missing-reason", func(d map[string]any) { delete(d, "incomplete_details") }, false},
		{"missing-id", func(d map[string]any) { delete(d, "id") }, false},
		{"missing-output", func(d map[string]any) { delete(d, "output") }, false},
		{"bad-text", func(d map[string]any) {
			d["output"] = []any{map[string]any{"type": "message", "content": []any{map[string]any{"type": "output_text", "text": 1}}}}
		}, false},
		{"upstream-error", func(d map[string]any) { d["error"] = map[string]any{"message": "failed"} }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := lengthResponse()
			tc.mutate(doc)
			before, _ := json.Marshal(doc)
			facts := NativeUsage("openai-responses")(doc, nil)
			if got := facts != nil && facts["pricing_blocked"] == nil; got != tc.valid {
				t.Fatalf("JSON cap结算准入=%v want=%v facts=%+v", got, tc.valid, facts)
			}
			var state NativeStreamState
			raw := responsesEvent("response.incomplete", doc)
			state.Observe("openai-responses", raw)
			if state.ResponseTerminal != tc.valid || state.Completed != tc.valid || state.Failed == tc.valid {
				t.Fatalf("SSE cap终态错误 %+v", state)
			}
			if tc.valid {
				u := NativeStreamUsage("openai-responses", raw, nil)
				if u["pricing_blocked"] != nil || u["output_tokens"] != float64(8) {
					t.Fatalf("合法终态用量丢失 %+v", u)
				}
			}
			after, _ := json.Marshal(doc)
			if string(before) != string(after) {
				t.Fatalf("终态校验修改原始响应 before=%s after=%s", before, after)
			}
			if doc["status"] != "incomplete" {
				t.Fatal("改写原生incomplete状态")
			}
		})
	}
}

// TestNativeResponsesTerminalBoundaries 验证完成状态匹配、历史失败粘性和仅完整快照才能提前关闭。
// 前置本地 SSE，保持旧 completed 空对象的观测兼容，但不允许它提前结束 HTTP；无外部资源。
func TestNativeResponsesTerminalBoundaries(t *testing.T) {
	for _, status := range []string{"completed", "incomplete"} {
		t.Run(status, func(t *testing.T) {
			doc := lengthResponse()
			doc["status"] = status
			var state NativeStreamState
			state.Observe("openai-responses", responsesEvent("response."+status, doc))
			if !state.Completed || !state.ResponseTerminal || state.Failed {
				t.Fatalf("完整终态未准入 %+v", state)
			}
			state.Observe("openai-responses", []byte("event: error\ndata: {}\n\n"))
			state.Observe("openai-responses", responsesEvent("response."+status, doc))
			if !state.Failed {
				t.Fatal("后续正常终态清除了既有失败")
			}
		})
	}
	t.Run("status-mismatch", func(t *testing.T) {
		doc := lengthResponse()
		doc["status"] = "completed"
		var state NativeStreamState
		state.Observe("openai-responses", responsesEvent("response.incomplete", doc))
		if !state.Failed || state.ResponseTerminal {
			t.Fatalf("不匹配状态被放行 %+v", state)
		}
	})
	t.Run("legacy-empty-completed", func(t *testing.T) {
		var state NativeStreamState
		state.Observe("openai-responses", []byte("event: response.completed\ndata: {}\n\n"))
		if !state.Completed || state.ResponseTerminal {
			t.Fatalf("不完整快照提前终止连接 %+v", state)
		}
	})
}

// TestNativeResponsesTerminalUsageRequired 验证终态独立缺失/坏计数不能被前帧补齐，合法累计仍可结算。
// 每例先发完整的中间用量，再发完成/长度终态；只在内存合并，无持久化清理。
func TestNativeResponsesTerminalUsageRequired(t *testing.T) {
	for _, status := range []string{"completed", "incomplete"} {
		for _, mode := range []string{"complete", "missing-all", "missing-output", "invalid", "conflict", "fraction"} {
			t.Run(status+"/"+mode, func(t *testing.T) {
				doc := lengthResponse()
				doc["status"] = status
				u := doc["usage"].(map[string]any)
				switch mode {
				case "missing-all":
					delete(doc, "usage")
				case "missing-output":
					delete(u, "output_tokens")
				case "invalid":
					u["output_tokens"] = "8"
				case "fraction":
					u["output_tokens"] = 1.5
				case "conflict":
					u["completion_tokens"] = 7
				}
				first := NativeStreamUsage("openai-responses", []byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"LENGTH_OK\",\"usage\":{\"input_tokens\":8,\"output_tokens\":2}}\n\n"), nil)
				final := NativeStreamUsage("openai-responses", responsesEvent("response."+status, doc), nil)
				got := llmdialogue.MergeDialogueUsage(first, final)
				if (got["pricing_blocked"] == nil) != (mode == "complete") {
					t.Fatalf("终态可靠性被前帧覆盖 mode=%s facts=%+v", mode, got)
				}
			})
		}
	}
}

// TestNativeIncompleteIsProtocolScoped 验证新增长度终态只作用于 Responses；图片错误事件不产生用量，保持既有失败状态。
// 参数 t 为上下文，本地 SSE 字节不连接外部服务，无清理资源。
func TestNativeIncompleteIsProtocolScoped(t *testing.T) {
	raw := responsesEvent("response.incomplete", lengthResponse())
	if facts := NativeStreamUsage("openai-images", raw, nil); facts != nil {
		t.Fatalf("图片协议误用Responses终态 %+v", facts)
	}
	var state NativeStreamState
	state.Observe("openai-images", raw)
	if !state.Failed || state.Completed || state.ResponseTerminal {
		t.Fatalf("非Responses失败语义改变 %+v", state)
	}
}
