package dialogue

// Turn 是文本与函数工具的内部对话项；ID 是工具调用关联 ID，Arguments 保留 JSON 参数字符串。
// 输入、上游与输出分别编码一次，避免按供应商和协议组合维护转换函数。
type Turn struct {
	Role, Text, ID, Name, Arguments string
	Calls                           []Turn
}

// Dialogue 保存统一对话请求；Options 只保存已实现协议可表达的已验证参数。
type Dialogue struct {
	Turns   []Turn
	Tools   []map[string]any
	Options map[string]any
	Stream  bool
}

// DialogueResult 保存实测回复与用量；ID 来源上游，由网关按该 ID 保存临时续接上下文。
type DialogueResult struct {
	ID, Text, Stop string
	Calls          []Turn
	Usage          map[string]any
}
