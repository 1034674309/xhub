package capability

import (
	"slices"
)

// DialogueProtocol 判断注册协议是否实现首版对话转换；参数为协议 ID，返回支持状态。
// 调用：部署校验和统一入口；媒体协议保持原执行器，不跨媒体转换。
func DialogueProtocol(protocol string) bool {
	return slices.Contains([]string{"openai-chat", "openai-responses", "anthropic-messages", "gemini", "vertex"}, protocol)
}
