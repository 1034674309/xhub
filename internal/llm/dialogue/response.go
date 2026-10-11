package dialogue

import (
	"encoding/json"
	"fmt"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
)

// ParseDialogueResult 提取上游文本、工具调用、结束原因和用量事实。参数为协议与响应；返回结果或格式错误。
// 调用：统一同步响应和流终态；不参与价格计算，不推断供应商。
func ParseDialogueResult(protocol string, raw []byte) (DialogueResult, error) {
	if protocol == "gemini" || protocol == "vertex" {
		return parseGoogleDialogueResult(raw)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return DialogueResult{}, err
	}
	if m["error"] != nil {
		return DialogueResult{}, fmt.Errorf("upstream dialogue error")
	}
	if protocol == "openai-responses" && m["status"] != "completed" && m["status"] != "incomplete" {
		return DialogueResult{}, fmt.Errorf("upstream response is not complete")
	}
	r := DialogueResult{ID: dialogueString(m["id"]), Usage: map[string]any{}}
	u := dialogueMap(m["usage"])
	r.Usage = NormalizeDialogueUsage(u)
	switch protocol {
	case "openai-chat":
		var result openai.ChatCompletion
		if err := json.Unmarshal(raw, &result); err != nil {
			return r, err
		}
		if len(result.Choices) != 1 {
			return r, fmt.Errorf("missing response choices")
		}
		if !dialogueFieldShape(result.JSON.Choices.Raw(), '[', false) {
			return r, fmt.Errorf("invalid upstream choices")
		}
		choice := result.Choices[0]
		// SDK 对未知字段和错误类型采用宽松解析；适配层必须拒绝错误内容，不能输出空白成功。
		content := choice.Message.JSON.Content
		if !dialogueFieldShape(choice.JSON.Message.Raw(), '{', false) || !dialogueFieldShape(content.Raw(), '"', true) {
			return r, fmt.Errorf("invalid upstream message content")
		}
		if choice.Message.Refusal != "" {
			return r, fmt.Errorf("unsupported response refusal")
		}
		tools := choice.Message.JSON.ToolCalls
		if !dialogueFieldShape(tools.Raw(), '[', true) {
			return r, fmt.Errorf("invalid upstream tool calls")
		}
		r.ID, r.Stop, r.Text = result.ID, choice.FinishReason, choice.Message.Content
		for _, call := range choice.Message.ToolCalls {
			if call.Type != "function" {
				return r, fmt.Errorf("unsupported response tool %s", call.Type)
			}
			r.Calls = append(r.Calls, Turn{ID: call.ID, Name: call.Function.Name, Arguments: call.Function.Arguments})
		}
	case "openai-responses":
		var result responses.Response
		if err := json.Unmarshal(raw, &result); err != nil {
			return r, err
		}
		r.ID = result.ID
		for _, item := range result.Output {
			switch item.Type {
			case "function_call":
				r.Calls = append(r.Calls, Turn{ID: item.CallID, Name: item.Name, Arguments: item.Arguments.OfString})
			case "message":
				if !dialogueFieldShape(item.JSON.Content.Raw(), '[', false) {
					return r, fmt.Errorf("invalid upstream message content")
				}
				for _, content := range item.Content {
					if content.Type != "output_text" {
						return r, fmt.Errorf("unsupported response content %s", content.Type)
					}
					if !dialogueFieldShape(content.JSON.Text.Raw(), '"', false) {
						return r, fmt.Errorf("invalid upstream output text")
					}
					r.Text += content.Text
				}
			default:
				return r, fmt.Errorf("unsupported response item %s", item.Type)
			}
		}
		r.Stop = "stop"
		if result.Status == "incomplete" {
			r.Stop = "length"
		}
	case "anthropic-messages":
		var result anthropic.Message
		if err := json.Unmarshal(raw, &result); err != nil {
			return r, err
		}
		r.ID = result.ID
		if !dialogueFieldShape(result.JSON.Content.Raw(), '[', false) {
			return r, fmt.Errorf("invalid upstream content blocks")
		}
		for _, block := range result.Content {
			switch block.Type {
			case "text":
				if !dialogueFieldShape(block.JSON.Text.Raw(), '"', false) {
					return r, fmt.Errorf("invalid upstream text block")
				}
				r.Text += block.Text
			case "tool_use":
				// SDK 保留原始 JSON 参数；共享协议只接受对象，避免传递无效工具输入。
				var args map[string]any
				if json.Unmarshal(block.Input, &args) != nil || args == nil {
					return r, fmt.Errorf("invalid upstream tool input")
				}
				encoded, _ := json.Marshal(args)
				r.Calls = append(r.Calls, Turn{ID: block.ID, Name: block.Name, Arguments: string(encoded)})
			default:
				return r, fmt.Errorf("unsupported response content %s", block.Type)
			}
		}
		r.Stop = string(result.StopReason)
		if r.Stop == "max_tokens" {
			r.Stop = "length"
		} else if r.Stop != "tool_use" {
			r.Stop = "stop"
		}
	default:
		return r, fmt.Errorf("unregistered dialogue protocol %s", protocol)
	}
	for _, call := range r.Calls {
		if call.ID == "" || call.Name == "" || !json.Valid([]byte(call.Arguments)) {
			return r, fmt.Errorf("invalid upstream tool call")
		}
	}
	if len(r.Calls) > 0 {
		r.Stop = "tool_calls"
	}
	return r, nil
}

// dialogueFieldShape 检查 SDK 保留字段的原始 JSON 形状，避免 SDK 将数字等强转字符串后丢失类型错误。
// 参数 raw 是已解析 JSON 字段，shape 是字符串、对象或数组的起始字节；nullable 允许缺失或 null。
// 返回是否符合适配契约；响应解析器调用，不重新实现供应商结构、不修改内容。
func dialogueFieldShape(raw string, shape byte, nullable bool) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nullable
	}
	return raw[0] == shape
}

// EncodeDialogueResult 将共享回复输出到用户协议；参数为结果、入口协议及公开型号，返回 JSON。
// 调用：统一同步入口及 SSE 终态；Responses 上游 store=false，网关通过临时上下文支持 previous_response_id。
func EncodeDialogueResult(r DialogueResult, protocol, model string) ([]byte, error) {
	if protocol == "gemini" || protocol == "vertex" {
		return encodeGoogleDialogueResult(r)
	}
	var out map[string]any
	u := r.Usage
	if u == nil {
		u = map[string]any{}
	}
	switch protocol {
	case "openai-chat":
		msg := map[string]any{"role": "assistant", "content": r.Text}
		if len(r.Calls) > 0 {
			var calls []any
			for _, c := range r.Calls {
				calls = append(calls, map[string]any{"id": c.ID, "type": "function", "function": map[string]any{"name": c.Name, "arguments": c.Arguments}})
			}
			msg["tool_calls"] = calls
		}
		out = map[string]any{"id": r.ID, "object": "chat.completion", "model": model, "choices": []any{map[string]any{"index": 0, "message": msg, "finish_reason": r.Stop}}, "usage": u}
	case "openai-responses":
		items := []any{}
		if r.Text != "" {
			items = append(items, map[string]any{"id": r.ID + "_message", "type": "message", "role": "assistant", "status": "completed", "content": []any{map[string]any{"type": "output_text", "text": r.Text, "annotations": []any{}}}})
		}
		for _, c := range r.Calls {
			items = append(items, map[string]any{"id": c.ID, "type": "function_call", "call_id": c.ID, "name": c.Name, "arguments": c.Arguments, "status": "completed"})
		}
		status := "completed"
		if r.Stop == "length" {
			status = "incomplete"
		}
		out = map[string]any{"id": r.ID, "object": "response", "model": model, "status": status, "store": false, "output": items, "usage": dialogueOutputUsage(u, protocol)}
		if status == "incomplete" {
			out["incomplete_details"] = map[string]any{"reason": "max_output_tokens"}
		}
	case "anthropic-messages":
		content := []any{}
		if r.Text != "" {
			content = append(content, map[string]any{"type": "text", "text": r.Text})
		}
		for _, c := range r.Calls {
			var args any
			if json.Unmarshal([]byte(c.Arguments), &args) != nil {
				return nil, fmt.Errorf("invalid response tool arguments")
			}
			content = append(content, map[string]any{"type": "tool_use", "id": c.ID, "name": c.Name, "input": args})
		}
		stop := "end_turn"
		if r.Stop == "length" {
			stop = "max_tokens"
		}
		if len(r.Calls) > 0 {
			stop = "tool_use"
		}
		out = map[string]any{"id": r.ID, "type": "message", "role": "assistant", "model": model, "content": content, "stop_reason": stop, "usage": dialogueOutputUsage(u, protocol)}
	default:
		return nil, fmt.Errorf("unregistered dialogue protocol %s", protocol)
	}
	return json.Marshal(out)
}
