package dialogue

import (
	"encoding/json"
	"fmt"

	llmdialogue "github.com/sunqirui1987/xhub/internal/llm/dialogue"
)

// responseReasoningItem 把完整 Responses 推理项输出为一个已关闭的 Messages 内容块。
// 参数为完整推理项及是否来自响应终态，返回字段/重复冲突/写出错误；consume 调用，不解密或复制厂商签名。
// 等到单项完成才发送封装；供应商可能在终态重新加密同一项，密文字节变化不能作为内容矛盾的证据。
func (s *dialogueStream) responseReasoningItem(item map[string]any, terminal bool) error {
	id := streamString(item["id"])
	if id == "" {
		return fmt.Errorf("reasoning item ID required")
	}
	block, err := llmdialogue.MessagesReasoningBlock(item)
	if err != nil {
		return err
	}
	raw, _ := json.Marshal(block)
	if previous, exists := s.responseReasoningBlocks[id]; exists {
		if previous != string(raw) {
			var before map[string]any
			_ = json.Unmarshal([]byte(previous), &before)
			// 按 LiteLLM 的 output_item.done 关闭一次内容块；终态仅补缺失项。
			// 相同明文可产生不同密文，已发出的封装保持原样；类型、公开摘要或有无密文的变化仍需拒绝。
			if !terminal || before["type"] != block["type"] || before["thinking"] != block["thinking"] ||
				(streamString(before["signature"]) != "") != (streamString(block["signature"]) != "") {
				return fmt.Errorf("conflicting completed reasoning item")
			}
		}
		return nil
	}
	s.responseReasoningBlocks[id] = string(raw)
	s.responseReasoningPending, s.responsePrelude = false, false
	if block == nil {
		return nil
	}
	if err := s.start(); err != nil {
		return err
	}
	if err := s.closeMessageBlock(); err != nil {
		return err
	}
	if block["type"] == "redacted_thinking" {
		index := s.nextBlock
		s.nextBlock++
		if err := s.emit("content_block_start", map[string]any{"type": "content_block_start", "index": index, "content_block": block}); err != nil {
			return err
		}
		return s.emit("content_block_stop", map[string]any{"type": "content_block_stop", "index": index})
	}
	if err := s.messageDelta(map[string]any{"type": "thinking", "thinking": "", "signature": ""},
		map[string]any{"type": "thinking_delta", "thinking": block["thinking"]}); err != nil {
		return err
	}
	if signature := streamString(block["signature"]); signature != "" {
		if err := s.emit("content_block_delta", map[string]any{"type": "content_block_delta", "index": s.messageBlockIndex,
			"delta": map[string]any{"type": "signature_delta", "signature": signature}}); err != nil {
			return err
		}
	}
	return s.closeMessageBlock()
}

// flushResponseText 释放推理项结束前暂存的正文；consume 在 output_item.done 或有效终态调用。
// 无参数，返回写入错误；标准 Responses 在推理结束后即恢复正文增量，缺省单项事件的上游等待终态校验。
func (s *dialogueStream) flushResponseText() error {
	text := s.pendingResponseText
	s.pendingResponseText = ""
	return s.textDelta(text)
}
