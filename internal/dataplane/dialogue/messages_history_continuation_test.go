package dialogue

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	llmdialogue "github.com/sunqirui1987/xhub/internal/llm/dialogue"
)

// TestMessagesHistoryThreeTurnContinuation 验证签名工具首轮、普通文字次轮后第三轮 previous_response_id 仍可恢复完整签名。
// 前置为受控引用缓存和内存 JSON/SSE，检查每轮保存 Responses 项、指令不继承、工具不重复且原生签名块完整，无外部数据清理。
func TestMessagesHistoryThreeTurnContinuation(t *testing.T) {
	for _, mode := range []string{"json", "sse"} {
		t.Run(mode, func(t *testing.T) {
			firstBody := map[string]any{"model": "public", "input": "Read both files", "instructions": "First only"}
			first, err := llmdialogue.ParseDialogue("openai-responses", firstBody)
			if err != nil {
				t.Fatal(err)
			}
			ref := llmdialogue.MessagesHistoryPrefix + strings.Repeat("e", 64)
			records := map[string]llmdialogue.MessagesReplay{}
			prepare := func(result llmdialogue.DialogueResult) (llmdialogue.DialogueResult, error) {
				if !llmdialogue.HasOpaqueMessagesResult(result) {
					return result, nil
				}
				prepared, err := llmdialogue.ResponsesWithMessagesHistory(result, ref)
				if err == nil {
					records[ref] = llmdialogue.MessagesReplay{Message: result.Native, Output: streamList(prepared.Native["output"])[1:]}
				}
				return prepared, err
			}
			run := func(blocks []any, dialogue llmdialogue.Dialogue, body map[string]any) []llmdialogue.Turn {
				t.Helper()
				if mode == "json" {
					stop := "end_turn"
					for _, value := range blocks {
						if streamMap(value)["type"] == "tool_use" {
							stop = "tool_use"
						}
					}
					raw, _ := json.Marshal(map[string]any{"id": "msg_test", "type": "message", "role": "assistant", "model": "claude", "content": blocks, "stop_reason": stop, "usage": map[string]any{"input_tokens": 10, "output_tokens": 12}})
					result, err := llmdialogue.ParseDialogueResult("anthropic-messages", raw)
					if err != nil {
						t.Fatal(err)
					}
					result, err = prepare(result)
					if err != nil {
						t.Fatal(err)
					}
					return ResponseHistory(dialogue, body, result)
				}
				var history []llmdialogue.Turn
				w := httptest.NewRecorder()
				_, _, _, _, err := PipeDialogueWithPreparation(w, &http.Response{Body: io.NopCloser(strings.NewReader(messagesHistoryFrames(t, blocks, true)))}, time.Now(), "public", "anthropic-messages", "openai-responses", nil, prepare, func(result llmdialogue.DialogueResult) { history = ResponseHistory(dialogue, body, result) })
				if err != nil {
					t.Fatal(err)
				}
				return history
			}
			blocks := messagesHistoryBlocks()
			firstHistory := run(blocks, first, firstBody)
			secondBody, err := ResponsesRequestBody(map[string]any{"model": "public", "previous_response_id": "first", "instructions": "Second only", "input": []any{map[string]any{"type": "function_call_output", "call_id": "c1", "output": "A"}, map[string]any{"type": "function_call_output", "call_id": "c2", "output": "B"}}}, firstHistory)
			if err != nil {
				t.Fatal(err)
			}
			second, err := llmdialogue.ParseResponsesWithMessagesHistory(secondBody, records)
			if err != nil {
				t.Fatal(err)
			}
			secondHistory := run([]any{map[string]any{"type": "text", "text": "Read complete"}}, second, secondBody)
			if len(secondHistory) == 0 {
				t.Fatal("普通文字次轮未保存上下文")
			}
			for _, turn := range secondHistory {
				if turn.NativeProtocol != "openai-responses" {
					t.Fatalf("次轮把原生Messages写入Responses历史: %#v", turn)
				}
			}
			thirdBody, err := ResponsesRequestBody(map[string]any{"model": "public", "previous_response_id": "second", "input": "What did you read?"}, secondHistory)
			if err != nil {
				t.Fatalf("普通次轮后无法第三轮续接: %v", err)
			}
			third, err := llmdialogue.ParseResponsesWithMessagesHistory(thirdBody, records)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := llmdialogue.EncodeDialogue(third, "anthropic-messages", "claude")
			if err != nil {
				t.Fatal(err)
			}
			messages := streamList(encoded["messages"])
			if len(messages) != 5 || !reflect.DeepEqual(streamMap(messages[1])["content"], blocks) {
				t.Fatalf("三轮签名或工具分组损坏: %#v", messages)
			}
			if streamMap(streamList(streamMap(messages[3])["content"])[0])["text"] != "Read complete" {
				t.Fatal("次轮普通回复丢失")
			}
			raw, _ := json.Marshal(encoded)
			if strings.Contains(string(raw), "First only") || strings.Contains(string(raw), "Second only") {
				t.Fatalf("临时指令跨轮继承: %s", raw)
			}
		})
	}
}

// TestResponsesHistoryOrdinaryResultBoundaries 验证普通 Chat/Messages 回复和拒绝说明保留为合法 Responses 历史，坏结果不发布半份历史。
// 前置为纯内存 Responses 输入，检查工具、公开拒绝、编码错误和空原生输出结构，无外部依赖或清理。
func TestResponsesHistoryOrdinaryResultBoundaries(t *testing.T) {
	input := map[string]any{"model": "public", "input": "Hello"}
	dialogue, err := llmdialogue.ParseDialogue("openai-responses", input)
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range []llmdialogue.DialogueResult{
		{ID: "chat", Text: "Hello back", Source: "openai-chat"},
		{ID: "refused", Refused: true, Refusal: "Request declined.", Source: "anthropic-messages"},
		{ID: "tool", Calls: []llmdialogue.Turn{{ID: "c", Name: "read", Arguments: "{}"}}},
	} {
		history := ResponseHistory(dialogue, input, result)
		next, err := ResponsesRequestBody(map[string]any{"input": "Continue"}, history)
		if err != nil {
			t.Fatalf("普通结果不能续接: %+v %v", result, err)
		}
		if _, err = llmdialogue.ParseDialogue("openai-responses", next); err != nil {
			t.Fatal(err)
		}
		if len(history) != 2 || history[1].NativeProtocol != "openai-responses" {
			t.Fatal("普通结果未保存合法Responses项")
		}
	}
	for _, result := range []llmdialogue.DialogueResult{
		{BridgeError: "unsupported upstream content"},
		{Source: "openai-responses", Native: map[string]any{"output": "invalid"}},
	} {
		if history := ResponseHistory(dialogue, input, result); history != nil {
			t.Fatalf("坏结果保存了不完整历史: %#v", history)
		}
	}
}
