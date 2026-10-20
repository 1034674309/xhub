package dialogue

import (
	"encoding/json"
	"fmt"
	llmdialogue "github.com/sunqirui1987/xhub/internal/llm/dialogue"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

// messagesHistoryFrames 为单条完整原生助手消息生成真实形状 SSE，签名和工具参数拆成多段。
// 参数 blocks 为原生块、complete 控制终态；返回独立事件文本，测试内存消费无需清理。
func messagesHistoryFrames(t *testing.T, blocks []any, complete bool) string {
	t.Helper()
	events := []map[string]any{{"type": "message_start", "message": map[string]any{"id": "msg_signed", "type": "message", "role": "assistant", "model": "claude", "content": []any{}, "usage": map[string]any{"input_tokens": 10, "output_tokens": 0}}}}
	tools := false
	for index, value := range blocks {
		original := streamMap(value)
		block := map[string]any{}
		for k, v := range original {
			block[k] = v
		}
		var deltas []map[string]any
		switch block["type"] {
		case "thinking":
			block["thinking"], block["signature"] = "", ""
			deltas = append(deltas, map[string]any{"type": "thinking_delta", "thinking": original["thinking"]})
			if signature, ok := original["signature"].(string); ok && signature != "" {
				deltas = append(deltas, map[string]any{"type": "signature_delta", "signature": signature[:len(signature)/2]}, map[string]any{"type": "signature_delta", "signature": signature[len(signature)/2:]})
			}
		case "text":
			block["text"] = ""
			deltas = append(deltas, map[string]any{"type": "text_delta", "text": original["text"]})
		case "tool_use":
			tools = true
			raw, _ := json.Marshal(original["input"])
			block["input"] = map[string]any{}
			deltas = append(deltas, map[string]any{"type": "input_json_delta", "partial_json": string(raw[:len(raw)/2])}, map[string]any{"type": "input_json_delta", "partial_json": string(raw[len(raw)/2:])})
		}
		events = append(events, map[string]any{"type": "content_block_start", "index": index, "content_block": block})
		for _, delta := range deltas {
			events = append(events, map[string]any{"type": "content_block_delta", "index": index, "delta": delta})
		}
		events = append(events, map[string]any{"type": "content_block_stop", "index": index})
	}
	if complete {
		stop := "end_turn"
		if tools {
			stop = "tool_use"
		}
		events = append(events, map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": stop}, "usage": map[string]any{"output_tokens": 12}}, map[string]any{"type": "message_stop"})
	}
	var output strings.Builder
	for _, event := range events {
		raw, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&output, "data: %s\n\n", raw)
	}
	return output.String()
}

// messagesHistoryBlocks 返回包含两份签名/隐藏推理、工具与交错文本的完整消息夹具，供流回放测试使用，无共享可变状态。
func messagesHistoryBlocks() []any {
	return []any{
		map[string]any{"type": "thinking", "thinking": "Public plan", "signature": "SECRET_SIGNATURE"},
		map[string]any{"type": "redacted_thinking", "data": "SECRET_REDACTED"},
		map[string]any{"type": "tool_use", "id": "c1", "name": "read", "input": map[string]any{"path": "a"}},
		map[string]any{"type": "text", "text": "Reading"},
		map[string]any{"type": "thinking", "thinking": "Second plan", "signature": "SECRET_SECOND"},
		map[string]any{"type": "tool_use", "id": "c2", "name": "read", "input": map[string]any{"path": "b"}},
	}
}

// historyStreamEvents 解码内存 SSE 以验证每个输出生命周期；参数为完整捕获，返回事件数组或测试失败，无外部副作用。
func historyStreamEvents(t *testing.T, raw []byte) []map[string]any {
	t.Helper()
	var events []map[string]any
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "data: ") {
			var event map[string]any
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
				t.Fatal(err)
			}
			events = append(events, event)
		}
	}
	return events
}

// TestMessagesHistoryStreamRoundTrip 验证签名分片、交错原生块、多个工具在完成前先持久化引用，done 与 completed 完全一致。
// 前置为真实形状内存 SSE；检查引用准备时尚无 done、完整历史恢复、签名不泄露以及 item ID 稳定，无外部资源清理。
func TestMessagesHistoryStreamRoundTrip(t *testing.T) {
	blocks := messagesHistoryBlocks()
	raw := messagesHistoryFrames(t, blocks, true)
	w := httptest.NewRecorder()
	ref := llmdialogue.MessagesHistoryPrefix + strings.Repeat("a", 64)
	preparedCount, commits := 0, 0
	var record llmdialogue.MessagesReplay
	var committed llmdialogue.DialogueResult
	prepare := func(result llmdialogue.DialogueResult) (llmdialogue.DialogueResult, error) {
		preparedCount++
		if strings.Contains(w.Body.String(), "output_item.done") {
			t.Fatal("引用保存晚于done")
		}
		if !reflect.DeepEqual(result.Native["content"], blocks) {
			t.Fatalf("签名分片或原始块顺序损坏: %#v", result.Native["content"])
		}
		result, err := llmdialogue.ResponsesWithMessagesHistory(result, ref)
		record = llmdialogue.MessagesReplay{Message: map[string]any{"role": "assistant", "content": blocks}, Output: streamList(result.Native["output"])[1:]}
		return result, err
	}
	_, _, _, output, err := PipeDialogueWithPreparation(w, &http.Response{Body: io.NopCloser(strings.NewReader(raw))}, time.Now(), "public", "anthropic-messages", "openai-responses", nil, prepare, func(result llmdialogue.DialogueResult) { commits++; committed = result })
	if err != nil {
		t.Fatalf("签名流失败: %v %s", err, output)
	}
	if preparedCount != 1 || commits != 1 || strings.Contains(string(output), "SECRET_") {
		t.Fatalf("准备/提交/泄密不正确: %d %d %s", preparedCount, commits, output)
	}
	done := map[int]map[string]any{}
	added := map[int]string{}
	var terminal map[string]any
	for _, event := range historyStreamEvents(t, output) {
		switch event["type"] {
		case "response.output_item.added":
			added[streamIndex(event["output_index"])] = streamString(streamMap(event["item"])["id"])
		case "response.output_item.done":
			index := streamIndex(event["output_index"])
			done[index] = streamMap(event["item"])
			if added[index] != done[index]["id"] {
				t.Fatalf("added/done ID不一致: %#v", event)
			}
		case "response.completed":
			terminal = streamMap(event["response"])
		}
	}
	items := streamList(terminal["output"])
	if len(items) != 4 || len(done) != 4 {
		t.Fatalf("输出索引缺失: %s", output)
	}
	for i, item := range items {
		if !reflect.DeepEqual(done[i], item) {
			t.Fatalf("done与终态不一致: %#v / %#v", done[i], item)
		}
	}
	if !reflect.DeepEqual(committed.Native["output"], items) {
		t.Fatal("缓存历史与客户端终态不同")
	}
	input := []any{map[string]any{"role": "user", "content": "Read files"}}
	input = append(input, items...)
	input = append(input, map[string]any{"type": "function_call_output", "call_id": "c1", "output": "A"}, map[string]any{"type": "function_call_output", "call_id": "c2", "output": "B"})
	dialogue, err := llmdialogue.ParseResponsesWithMessagesHistory(map[string]any{"input": input, "model": "public"}, map[string]llmdialogue.MessagesReplay{ref: record})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := llmdialogue.EncodeDialogue(dialogue, "anthropic-messages", "claude")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(streamMap(streamList(encoded["messages"])[1])["content"], blocks) {
		t.Fatal("工具续轮未恢复完整原生内容")
	}
}

// TestMessagesHistoryStreamFailures 验证未配置准备器、缓存失败、截断、迟到thinking均不发布成功done或终态。
// 前置为内存SSE和可失败准备函数；检查明确失败与禁止成功提交，无付费调用或清理数据。
func TestMessagesHistoryStreamFailures(t *testing.T) {
	for _, name := range []string{"no_preparer", "cache_failure", "truncated", "late_thinking"} {
		t.Run(name, func(t *testing.T) {
			blocks := messagesHistoryBlocks()
			if name == "late_thinking" {
				blocks = append([]any{map[string]any{"type": "text", "text": "Already visible"}}, blocks...)
			}
			raw := messagesHistoryFrames(t, blocks, name != "truncated")
			w := httptest.NewRecorder()
			prepared, committed := 0, 0
			var prepare func(llmdialogue.DialogueResult) (llmdialogue.DialogueResult, error)
			if name != "no_preparer" {
				prepare = func(r llmdialogue.DialogueResult) (llmdialogue.DialogueResult, error) {
					prepared++
					return r, fmt.Errorf("cache unavailable")
				}
			}
			_, _, _, output, err := PipeDialogueWithPreparation(w, &http.Response{Body: io.NopCloser(strings.NewReader(raw))}, time.Now(), "public", "anthropic-messages", "openai-responses", nil, prepare, func(llmdialogue.DialogueResult) { committed++ })
			if err == nil || committed != 0 || strings.Contains(string(output), "output_item.done") || strings.Contains(string(output), "response.completed") {
				t.Fatalf("失败流伪装成功: %v %s", err, output)
			}
			if name != "cache_failure" && prepared != 0 {
				t.Fatal("完整校验前调用持久化")
			}
		})
	}
}

// TestMessagesHistoryNormalStream 验证启用准备器后普通文本不新增空推理项，公开无签名thinking仍可完成。
// 前置为两类内存消息，准备器保留原结果；检查无索引洞、无引用、文本输出，无外部清理。
func TestMessagesHistoryNormalStream(t *testing.T) {
	for _, thinking := range []bool{false, true} {
		t.Run(fmt.Sprintf("thinking=%v", thinking), func(t *testing.T) {
			blocks := []any{}
			if thinking {
				blocks = append(blocks, map[string]any{"type": "thinking", "thinking": "Public", "signature": ""})
			}
			blocks = append(blocks, map[string]any{"type": "text", "text": "Hello"})
			w := httptest.NewRecorder()
			_, _, _, output, err := PipeDialogueWithPreparation(w, &http.Response{Body: io.NopCloser(strings.NewReader(messagesHistoryFrames(t, blocks, true)))}, time.Now(), "public", "anthropic-messages", "openai-responses", nil, func(r llmdialogue.DialogueResult) (llmdialogue.DialogueResult, error) { return r, nil })
			if err != nil {
				t.Fatalf("普通流失败: %v %s", err, output)
			}
			for _, event := range historyStreamEvents(t, output) {
				if event["type"] == "response.completed" {
					items := streamList(streamMap(event["response"])["output"])
					want := 1
					if thinking {
						want = 2
					}
					if len(items) != want {
						t.Fatalf("普通流新增空项: %s", output)
					}
					for _, item := range items {
						if item == nil || streamMap(item)["encrypted_content"] != nil {
							t.Fatal("普通流产生空洞或引用")
						}
					}
				}
			}
		})
	}
}
