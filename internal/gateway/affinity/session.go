package affinity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"unicode"
)

// sessionSignal 严格校验客户端信号，供 explicitSession/PlanRoute 校验会话、缓存键及父响应 ID。
// 参数 raw 是 JSON 动态值或 HTTP 头字符串；成功返回原字符串和 nil，失败返回空串及错误。
// 只接受非空 string，上限 256 字节（不是字符数），不得有首尾空白或 Unicode 控制字符。
// 不清理后接受、不转换数字类型、不修改输入；内部普通空格不在此禁止。
// 字段缺失是否允许由调用方决定，存在但为空或 nil 必须判为非法；无存储副作用。
func sessionSignal(raw any) (string, error) {
	value, ok := raw.(string)
	if !ok || value == "" || len(value) > 256 || strings.TrimSpace(value) != value {
		return "", fmt.Errorf("expected a nonempty string of at most 256 bytes without surrounding whitespace")
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("control characters are not allowed")
		}
	}
	return value, nil
}

// explicitSession 合并明确会话信号，供 PlanRoute 在推断和父响应继承之前调用。
// 参数 r 为 HTTP 请求、body 为解析正文，两者可 nil；返回明确 ID 或错误。
// 来源仅为所有 X-Session-Id 头值和正文 session_id；无来源返回空串和 nil，允许继续推断。
// 每个来源由 sessionSignal 校验，多来源必须完全同值；非法值或冲突返回错误。
// 重复头按 Header.Values 的各值处理，不自行按逗号拆分；旧 session-id 头和 metadata 不参与。
// 只读输入，不填充或删除正文；明确 ID 是会话信号，不是访问历史的授权凭据。
func explicitSession(r *http.Request, body map[string]any) (string, error) {
	var values []any
	if r != nil {
		for _, v := range r.Header.Values("X-Session-Id") {
			values = append(values, v)
		}
	}
	if v, exists := body["session_id"]; exists {
		values = append(values, v)
	}
	var id string
	for _, raw := range values {
		v, err := sessionSignal(raw)
		if err != nil {
			return "", fmt.Errorf("invalid session_id: %w", err)
		}
		if id != "" && id != v {
			return "", fmt.Errorf("conflicting session IDs")
		}
		id = v
	}
	return id, nil
}

// fingerprint 将结构编码后计算指纹，供缓存键和首轮内容推断会话。
// 参数 prefix 为输出命名空间（例如 cache:v1:），value 为参与摘要的完整结构。
// 返回 prefix 加 64 位 SHA256 十六进制摘要；JSON 编码失败返回空串，不报错或记录明文。
// prefix 拼在摘要前，不参与哈希；数组顺序、原始空白及字段内容参与摘要。
// 不读取媒体、不访问缓存、不修改输入；哈希仅是推断信号，不提供租户隔离或权限。
func fingerprint(prefix string, value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return prefix + hex.EncodeToString(sum[:])
}

// newSessionID 生成随机 UUID v4，供 PlanRoute 创建会话 ID 和偏好写入的 Revision。
// 无参数，返回带连字符的 UUID；crypto/rand 读取 16 字节，再设置版本位和变体位。
// 随机版本区分“部署相同但记录已变化”的写入，避免比较更新的 ABA 问题。
// 随机源失败会 panic，不退回时间戳等可能重复的标识；不访问缓存或共享状态。
func newSessionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	// UUID v4 的高四位标记版本，另一字节高两位标记 RFC 变体，不改变其余随机位。
	b[6] = b[6]&15 | 64
	b[8] = b[8]&63 | 128
	h := hex.EncodeToString(b[:])
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

// stableContent 判断内容能否完整参与会话指纹，供 contentSessionID 处理指令和用户消息。
// 参数 value 是字符串或 JSON 内容块数组；返回原内容和 true，不能可靠识别则返回 nil、false。
// 字符串需有非空白内容；数组必须非空，各块为对象、类型已知、必要文本或媒体字段为非空字符串。
// 支持现有文本、图片、音频、文档、文件形态；只检查必要字段，不做完整协议校验、媒体解码或 URL 请求。
// 任一未知或不完整块会使整段拒绝推断，避免仅用共用文本把不同媒体会话合并。
// 成功返回原数组而非字段投影，附加字段、块顺序及完整媒体结构也参与摘要，不复制或修改输入。
func stableContent(value any) (any, bool) {
	if text, ok := value.(string); ok {
		return text, strings.TrimSpace(text) != ""
	}
	parts, ok := value.([]any)
	if !ok || len(parts) == 0 {
		return nil, false
	}
	for _, raw := range parts {
		part, ok := raw.(map[string]any)
		if !ok {
			return nil, false
		}
		// 必要字段按各协议的 JSON 形态检查；任一块失败即停止，绝不只哈希识别出来的部分。
		switch asString(part["type"]) {
		case "text", "input_text":
			if strings.TrimSpace(asString(part["text"])) == "" {
				return nil, false
			}
		case "image_url":
			image, _ := part["image_url"].(map[string]any)
			if asString(image["url"]) == "" {
				return nil, false
			}
		case "input_image":
			if asString(part["image_url"]) == "" && asString(part["file_id"]) == "" {
				return nil, false
			}
		case "input_audio":
			audio, _ := part["input_audio"].(map[string]any)
			if asString(audio["data"]) == "" || asString(audio["format"]) == "" {
				return nil, false
			}
		case "image", "document":
			source, _ := part["source"].(map[string]any)
			if asString(source["data"]) == "" && asString(source["url"]) == "" {
				return nil, false
			}
		case "input_file":
			if asString(part["file_id"]) == "" && asString(part["file_data"]) == "" && asString(part["file_url"]) == "" {
				return nil, false
			}
		default:
			return nil, false
		}
	}
	return parts, true
}

// contentSessionID 从开头指令和首条用户内容推断稳定会话，供 PlanRoute 无明确信号时调用。
// 参数 body 为解析正文，可 nil；返回 prompt:v1: 指纹，不能可靠识别则返回空字符串。
// 先按 system、instructions 固定顺序收集顶层指令，再读 messages（字段不存在时才改读 input）。
// 字符串输入视为用户消息；数组只接受首条 user 之前的 system/developer，找到 user 即计算并返回。
// 指令或首条用户内容不完整、此前出现其他角色/结构、仅有指令而无 user，均拒绝推断。
// 找到 user 后不再读取后续历史，追加 assistant/user 或工具轮次不改变指纹。
// 顶层 model、tools 和入口不参与内容摘要；模型及入口隔离由缓存键另行保证。
// 不修改正文；空返回值允许 PlanRoute 生成新 ID，不是请求验证错误。
func contentSessionID(body map[string]any) string {
	if body == nil {
		return ""
	}
	// 顶层指令也参与首轮身份；存在但不完整就拒绝推断，不忽略后继续用用户文本。
	var prefix []any
	for _, key := range []string{"system", "instructions"} {
		if value, exists := body[key]; exists {
			content, ok := stableContent(value)
			if !ok {
				return ""
			}
			prefix = append(prefix, map[string]any{"role": key, "content": content})
		}
	}
	// messages 的存在性决定入口优先级，即使为空或类型错误，也不改用 input 掩盖问题。
	messages, exists := body["messages"]
	if !exists {
		messages = body["input"]
	}
	if input, ok := messages.(string); ok {
		if strings.TrimSpace(input) == "" {
			return ""
		}
		return fingerprint("prompt:v1:", append(prefix, map[string]any{"role": "user", "content": input}))
	}
	list, _ := messages.([]any)
	for _, raw := range list {
		message, ok := raw.(map[string]any)
		if !ok {
			return ""
		}
		role := asString(message["role"])
		if role == "system" || role == "developer" || role == "user" {
			content, ok := stableContent(message["content"])
			if !ok {
				return ""
			}
			prefix = append(prefix, map[string]any{"role": role, "content": content})
			// 首条用户内容划定指纹边界，后续轮次无需重算且不会参与摘要。
			if role == "user" {
				return fingerprint("prompt:v1:", prefix)
			}
		} else {
			return ""
		}
	}
	return ""
}
