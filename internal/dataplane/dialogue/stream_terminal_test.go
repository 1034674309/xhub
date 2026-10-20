package dialogue

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	llmdialogue "github.com/sunqirui1987/xhub/internal/llm/dialogue"
)

// terminalBoundaryBody 在完整SSE后拒绝继续读取，模拟终态已到但HTTP正文仍打开；Close记录资源释放，无外部资源。
type terminalBoundaryBody struct {
	*strings.Reader
	closed bool
}

// Read 只提供已有事件；事件读尽后返回确定错误，保证测试不会用EOF掩盖多读终态之后正文的问题。
func (b *terminalBoundaryBody) Read(p []byte) (int, error) {
	if b.Len() == 0 {
		return 0, errors.New("read beyond semantic terminal")
	}
	return b.Reader.Read(p)
}

// Close 标记上游已释放；供PipeDialogue生命周期断言，无其他副作用。
func (b *terminalBoundaryBody) Close() error { b.closed = true; return nil }

// terminalTestFrames 生成带完整工具参数及用量的来源协议SSE；stop控制正常/截断，返回分帧文本用于单元边界测试。
func terminalTestFrames(source, stop string) string {
	events := []string{}
	switch source {
	case "openai-chat":
		events = []string{`{"id":"chat_terminal","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_terminal","type":"function","function":{"name":"lookup","arguments":"{}"}}]},"finish_reason":"` + stop + `"}],"usage":{"prompt_tokens":8,"completion_tokens":2}}`, "[DONE]"}
	case "openai-responses":
		status := "completed"
		if stop == "length" {
			status = "incomplete"
		}
		events = []string{`{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","call_id":"call_terminal","name":"lookup","arguments":"{}"}}`, `{"type":"response.` + status + `","response":{"id":"resp_terminal","status":"` + status + `","output":[{"type":"function_call","call_id":"call_terminal","name":"lookup","arguments":"{}"}],"usage":{"input_tokens":8,"output_tokens":2}}}`}
	case "anthropic-messages":
		if stop == "length" {
			stop = "max_tokens"
		}
		events = []string{`{"type":"message_start","message":{"id":"msg_terminal","type":"message","role":"assistant","content":[],"usage":{"input_tokens":8}}}`, `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"call_terminal","name":"lookup","input":{}}}`, `{"type":"content_block_stop","index":0}`, `{"type":"message_delta","delta":{"stop_reason":"` + stop + `"},"usage":{"output_tokens":2}}`, `{"type":"message_stop"}`}
	}
	return "data: " + strings.Join(events, "\n\ndata: ") + "\n\n"
}

// TestSemanticTerminalStopsReading 验证三协议正常/截断工具终态立即结束且只提交一次，不依赖HTTP EOF。
// 参数t为上下文；内存上游尾读必失败，核对工具ID、usage、停止原因和正文关闭；无数据库或网络需清理。
func TestSemanticTerminalStopsReading(t *testing.T) {
	for _, source := range []string{"openai-chat", "openai-responses", "anthropic-messages"} {
		for _, target := range []string{"openai-chat", "openai-responses", "anthropic-messages"} {
			for _, stop := range []string{"tool_calls", "length"} {
				t.Run(source+"/"+target+"/"+stop, func(t *testing.T) {
					body := &terminalBoundaryBody{Reader: strings.NewReader(terminalTestFrames(source, stop))}
					w := httptest.NewRecorder()
					commits := 0
					_, usage, _, raw, err := PipeDialogue(w, &http.Response{Body: body}, time.Now(), "public", source, target, func(r llmdialogue.DialogueResult) {
						commits++
						if stop == "length" && r.Stop != "length" {
							t.Errorf("truncation lost at commit: %s", r.Stop)
						}
					})
					if !strings.Contains(w.Header().Get("Cache-Control"), "no-transform") {
						t.Fatal("SSE终态链缺少代理防缓冲响应头")
					}
					// 原生Chat在外层执行器结算后提交归属，其余路径在终态可见前提交。
					expectedCommits := 1
					if source == "openai-chat" && target == source {
						expectedCommits = 0
					}
					if err != nil || !body.closed || commits != expectedCommits || usage["total_tokens"] != float64(10) || !strings.Contains(string(raw), "call_terminal") {
						t.Fatalf("terminal contract err=%v closed=%v commits=%d usage=%v output=%s", err, body.closed, commits, usage, raw)
					}
					if stop == "length" {
						want := map[string]string{"openai-chat": `"finish_reason":"length"`, "openai-responses": `"status":"incomplete"`, "anthropic-messages": `"stop_reason":"max_tokens"`}[target]
						if !strings.Contains(string(raw), want) {
							t.Fatalf("truncated tool reported success: %s", raw)
						}
					}
				})
			}
		}
	}
}

// TestMalformedTerminalNeverCommits 验证缺停止原因、缺终态对象、状态冲突和损坏工具均显式失败，不发布成功或归属。
// 参数t为上下文；正常文本增量后注入异常终态，覆盖所有入口，内存资源由Pipe关闭，无持久化清理。
func TestMalformedTerminalNeverCommits(t *testing.T) {
	cases := []struct{ name, source, frames string }{
		{"chat_missing_reason", "openai-chat", "data: {\"id\":\"bad\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"partial\"}}]}\n\ndata: [DONE]\n\n"},
		{"responses_missing_object", "openai-responses", "data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n\ndata: {\"type\":\"response.completed\"}\n\n"},
		{"responses_missing_output", "openai-responses", "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"bad\",\"status\":\"completed\"}}\n\n"},
		{"responses_status_conflict", "openai-responses", "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"bad\",\"status\":\"incomplete\",\"output\":[]}}\n\n"},
		{"responses_missing_id", "openai-responses", "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"output\":[]}}\n\n"},
		{"messages_missing_reason", "anthropic-messages", "data: {\"type\":\"message_start\",\"message\":{\"id\":\"bad\"}}\n\ndata: {\"type\":\"message_stop\"}\n\n"},
		{"incomplete_tool", "openai-chat", strings.Replace(terminalTestFrames("openai-chat", "length"), `"arguments":"{}"`, `"arguments":"{"`, 1)},
	}
	for _, tc := range cases {
		for _, target := range []string{"openai-chat", "openai-responses", "anthropic-messages"} {
			t.Run(tc.name+"/"+target, func(t *testing.T) {
				commits := 0
				_, _, _, raw, err := PipeDialogue(httptest.NewRecorder(), &http.Response{Body: io.NopCloser(strings.NewReader(tc.frames))}, time.Now(), "public", tc.source, target, func(llmdialogue.DialogueResult) { commits++ })
				if err == nil || commits != 0 {
					t.Fatalf("invalid terminal committed: err=%v commits=%d body=%s", err, commits, raw)
				}
				for _, line := range strings.Split(string(raw), "\n") {
					if strings.HasPrefix(line, "data:") {
						var event map[string]any
						_ = json.Unmarshal([]byte(strings.TrimPrefix(line, "data:")), &event)
						if event["type"] == "response.completed" || event["type"] == "message_stop" {
							t.Fatalf("invalid terminal emitted success: %s", raw)
						}
					}
				}
			})
		}
	}
}

// TestNativeThinkingObservationBoundary 验证原生观察允许厂商思考事件，共享转换仍明确拒绝；不合成文本或签名，纯状态无清理。
func TestNativeThinkingObservationBoundary(t *testing.T) {
	for _, frame := range []string{`{"type":"content_block_start","content_block":{"type":"thinking","thinking":"","signature":""}}`, `{"type":"content_block_start","content_block":{"type":"redacted_thinking","data":"opaque"}}`, `{"type":"content_block_delta","delta":{"type":"thinking_delta","thinking":"plan"}}`, `{"type":"content_block_delta","delta":{"type":"signature_delta","signature":"opaque"}}`} {
		for _, observe := range []bool{true, false} {
			s := dialogueStream{observeOnly: observe, usage: map[string]any{}}
			if err := s.consume("anthropic-messages", frame); (err == nil) != observe {
				t.Errorf("native observation=%v frame=%s err=%v", observe, frame, err)
			}
		}
	}
}
