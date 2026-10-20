package dialogue

import "fmt"

// projectMessagesCitations 处理 Messages 文本块的引用占位，参数是已复制的共享投影块。
// ParseDialogue 调用；返回非空引用的桥接限制或结构错误，不修改保存于 Dialogue.Native 的原文。
// null 和空数组没有引用语义，可安全从共享投影删除；真实引用只允许原生重放，不能跨协议静默丢失。
func projectMessagesCitations(block map[string]any) (string, error) {
	if block["type"] != "text" {
		return "", nil
	}
	value, exists := block["citations"]
	if !exists {
		return "", nil
	}
	if value == nil {
		delete(block, "citations")
		return "", nil
	}
	citations, ok := value.([]any)
	if !ok {
		return "", fmt.Errorf("text citations must be an array or null")
	}
	for _, citation := range citations {
		if dialogueMap(citation) == nil {
			return "", fmt.Errorf("text citation must be an object")
		}
	}
	delete(block, "citations")
	if len(citations) > 0 {
		// 引用类型及定位信息由原生上游校验；共享对话尚无可等价表达的位置和文档身份。
		return "Messages citations require native Messages", nil
	}
	return "", nil
}
