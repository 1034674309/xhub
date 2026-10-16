package affinity

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// claudeSessionID 读取 Claude Code 的原生会话信号，供 Messages 入口在首轮内容指纹之前调用。
// 参数 r 为原请求、body 为只读正文；返回会话 ID 或重复/非法头错误，无信号返回空串。
// 参考 LiteLLM：专用会话头优先，其次 metadata.user_id 的 JSON session_id 或旧版复合串。
// 普通用户 ID 不能代表会话；这里不改变正文、不信任 device/account 身份，也不提供历史访问权限。
func claudeSessionID(r *http.Request, body map[string]any) (string, error) {
	var id string
	if r != nil {
		for _, raw := range r.Header.Values("X-Claude-Code-Session-Id") {
			value := claudeSessionValue(raw)
			if value == "" {
				return "", fmt.Errorf("invalid Claude Code session ID")
			}
			if id != "" && id != value {
				return "", fmt.Errorf("conflicting Claude Code session IDs")
			}
			id = value
		}
	}
	if id != "" {
		return id, nil
	}
	metadata, _ := body["metadata"].(map[string]any)
	user, ok := metadata["user_id"].(string)
	if !ok || len(user) > 4096 {
		return "", nil
	}
	// 新版 Claude 把 device/account/session 编码在字符串中；只提取会话，避免设备上的并行对话被合并。
	var decoded map[string]any
	if json.Unmarshal([]byte(user), &decoded) == nil {
		return claudeSessionValue(decoded["session_id"]), nil
	}
	// 旧客户端使用 user_<device>_account_<account>_session_<id>；不把任意含 session 的普通用户名当会话。
	if strings.HasPrefix(user, "user_") && strings.Contains(user, "_account_") {
		if index := strings.LastIndex(user, "_session_"); index >= 0 {
			return claudeSessionValue(user[index+len("_session_"):]), nil
		}
	}
	return "", nil
}

// claudeSessionValue 校验客户端会话候选；参数为动态值，返回有效原字符串或空串。
// claudeSessionID 共用此校验；仅接受至多 256 字节的 ASCII 字母、数字、连字符及下划线，不截短、不修改输入。
func claudeSessionValue(raw any) string {
	value, ok := raw.(string)
	if !ok || value == "" || len(value) > 256 {
		return ""
	}
	for _, char := range value {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' || char == '_') {
			return ""
		}
	}
	return value
}
