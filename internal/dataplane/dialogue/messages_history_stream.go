package dialogue

import (
	"fmt"
	llmdialogue "github.com/sunqirui1987/xhub/internal/llm/dialogue"
)

// streamList 读取已经验证的 JSON 数组，返回 nil 表示类型不符；完成投影调用，不修改或隐式转换。
func streamList(value any) []any { result, _ := value.([]any); return result }

// startMessagesReasoning 为 Messages 首个推理块预留一个 Responses 引用项，并保持后续正文/工具索引稳定。
// 无参数，返回顺序或写入错误；只在受控桥接调用，多个原生推理块共用完整助手消息引用。
// 若正文已可见，不能追插 leading 项，故明确拒绝；没有推理的普通流从不调用，不生成空 reasoning。
func (s *dialogueStream) startMessagesReasoning() error {
	if s.messagesReasoning {
		return nil
	}
	if s.nextBlock != 0 {
		return fmt.Errorf("Messages thinking must precede visible content for Responses history")
	}
	if err := s.start(); err != nil {
		return err
	}
	s.messagesReasoning = true
	s.nextBlock = 1
	return s.emit("response.output_item.added", map[string]any{"type": "response.output_item.added", "output_index": 0, "item": map[string]any{"id": llmdialogue.ResponseItemID("rs", s.responseItemSeed), "type": "reasoning", "summary": []any{}}})
}
