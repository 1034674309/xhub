package dialogue

import (
	"fmt"
	"strings"
)

// ResponsesReasoningSignaturePrefix 区分网关封装的 Responses 密文与 Anthropic 原厂签名。
// 参考 LiteLLM encrypted_reasoning_signature 的不透明往返；不是签名生成或解密，连接归属由路由层校验。
const ResponsesReasoningSignaturePrefix = "xhub_responses_reasoning:"

// IsResponsesReasoningSignature 判断任意客户端值是否声明网关密文封装；路由层据此要求固定连接。
// 参数为 signature/data，返回是否有保留前缀；空载荷也返回 true，随后解析器会拒绝，不能降级为普通请求。
func IsResponsesReasoningSignature(value any) bool {
	s, ok := value.(string)
	return ok && strings.HasPrefix(s, ResponsesReasoningSignaturePrefix)
}

// MessagesReasoningBlock 将一项 Responses 推理转换为 Messages thinking 或 redacted_thinking。
// 参数为上游原生项，返回独立内容块、无可见内容时的 nil 或字段错误；同步和 SSE 共用，无输入修改。
// 公开摘要与密文分别保存，不把密文展示为思维文本，也不伪造 Anthropic 的厂商签名。
func MessagesReasoningBlock(item map[string]any) (map[string]any, error) {
	var parts []string
	if value, exists := item["summary"]; exists {
		list, ok := value.([]any)
		if !ok {
			return nil, fmt.Errorf("invalid reasoning summary")
		}
		for _, value := range list {
			part := dialogueMap(value)
			text, ok := part["text"].(string)
			if !ok || part["type"] != "summary_text" {
				return nil, fmt.Errorf("invalid reasoning summary text")
			}
			parts = append(parts, text)
		}
	}
	text := strings.Join(parts, "\n\n")
	cipher := ""
	if value := item["encrypted_content"]; value != nil {
		var ok bool
		cipher, ok = value.(string)
		if !ok {
			return nil, fmt.Errorf("invalid encrypted reasoning content")
		}
	}
	if cipher != "" {
		if text == "" {
			return map[string]any{"type": "redacted_thinking", "data": ResponsesReasoningSignaturePrefix + cipher}, nil
		}
		return map[string]any{"type": "thinking", "thinking": text, "signature": ResponsesReasoningSignaturePrefix + cipher}, nil
	}
	if text == "" {
		return nil, nil
	}
	return map[string]any{"type": "thinking", "thinking": text, "signature": ""}, nil
}

// messagesReasoningReplay 还原网关封装的单个 thinking/redacted 块为 Responses 原生历史项。
// 参数为客户端内容块和角色，返回受限 Turn 或错误；只接受本网关命名空间，原厂签名仍被拒绝。
// 与 LiteLLM 一样省略重建项的 ID，避免把虚构 ID 或跨账户 ID 传给上游；密文原字节保留。
func messagesReasoningReplay(block map[string]any, role string) (Turn, error) {
	if role != "assistant" {
		return Turn{}, fmt.Errorf("thinking requires assistant text")
	}
	value := block["signature"]
	text := ""
	if block["type"] == "redacted_thinking" {
		value = block["data"]
	} else {
		var ok bool
		text, ok = block["thinking"].(string)
		if !ok {
			return Turn{}, fmt.Errorf("thinking requires assistant text")
		}
	}
	if value == nil || value == "" {
		if block["type"] == "redacted_thinking" {
			return Turn{}, fmt.Errorf("redacted thinking requires Responses envelope")
		}
		return Turn{Role: "reasoning", Reasoning: text}, nil
	}
	if !IsResponsesReasoningSignature(value) {
		return Turn{}, fmt.Errorf("signed thinking requires native Messages")
	}
	cipher := strings.TrimPrefix(value.(string), ResponsesReasoningSignaturePrefix)
	if cipher == "" {
		return Turn{}, fmt.Errorf("encrypted reasoning envelope is empty")
	}
	summary := []any{}
	if text != "" {
		summary = append(summary, map[string]any{"type": "summary_text", "text": text})
	}
	return Turn{Role: "reasoning", NativeProtocol: "openai-responses", Native: map[string]any{
		"type": "reasoning", "summary": summary, "encrypted_content": cipher,
	}}, nil
}

// messagesReasoningContent 为同步 Messages 输出收集保持分项边界的推理块。
// 参数为已校验上游结果，返回内容数组或错误；Chat 沿用公开推理，Responses 按原项封装，调用方不持有共享引用。
func messagesReasoningContent(result DialogueResult) ([]any, error) {
	content := []any{}
	if result.Source == "openai-responses" {
		for _, value := range dialogueList(result.Native["output"]) {
			item := dialogueMap(value)
			if item["type"] != "reasoning" {
				continue
			}
			block, err := MessagesReasoningBlock(item)
			if err != nil {
				return nil, err
			}
			if block != nil {
				content = append(content, block)
			}
		}
	} else if result.Reasoning != "" {
		content = append(content, map[string]any{"type": "thinking", "thinking": result.Reasoning, "signature": ""})
	}
	return content, nil
}
