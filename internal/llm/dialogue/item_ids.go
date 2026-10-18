package dialogue

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// responseUUID 只识别 Grok 实际返回的完整 UUID；不把任意 _message/_reasoning 后缀误判为旧网关产物。
var responseUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// ResponseTextItemID 为网关合成的 Responses 正文或公开推理项生成稳定 ID。
// 参数 prefix 是调用方固定的 msg/rs 类型前缀，responseID 是上游响应级 ID；返回类型前缀、xhub 命名空间和十六进制摘要组成的 ID。
// 同步与 SSE 编码器共用；xhub 命名空间标识合成项，空或特殊上游 ID 不直接拼入结果，不修改原生 ID/工具 call_id。
func ResponseTextItemID(prefix, responseID string) string {
	sum := sha256.Sum256([]byte("xhub:responses-text:v1\x00" + prefix + "\x00" + responseID))
	return prefix + "_xhub_" + hex.EncodeToString(sum[:16])
}

// normalizeLegacyResponsesTextIDs 修复旧 Chat/Grok 桥接正文 ID，并规范公开推理历史。
// 参数 body 是已复制的 Responses 正文；原地处理完整公开项，无返回值、无外部副作用。
// 公开摘要不是原厂存储引用；仅改成 rs_ 前缀仍会触发 item not found，因此回放时省略合成 ID、保留摘要。
// Grok 原生公开摘要的 rs_UUID/status 在严格原生目标触发存储引用/字段错误；仅对已识别完整公开项省略这两项元数据。
// 其他原生 ID、非字符串 ID、加密项和 item_reference 均不改写；被引用项也保持原样，避免把引用伪造成新归属。
func normalizeLegacyResponsesTextIDs(body map[string]any) {
	items := dialogueList(body["input"])
	references := map[string]bool{}
	for _, value := range items {
		item := dialogueMap(value)
		if item["type"] == "item_reference" {
			references[dialogueString(item["id"])] = true
		}
	}
	for _, value := range items {
		item := dialogueMap(value)
		id := dialogueString(item["id"])
		if references[id] || item["encrypted_content"] != nil {
			continue
		}
		if item["type"] == "reasoning" && strings.HasPrefix(id, "rs_xhub_") && len(id) == len("rs_xhub_")+32 {
			delete(item, "id")
			if item["status"] == nil || item["status"] == "completed" {
				delete(item, "status")
			}
			continue
		}
		// 完整公开 Grok 摘要可跨厂商回放，但其 UUID 不能被解释成 OpenAI 存储引用。
		// 只规范已完成或空状态；未知状态、引用项和密文仍交原协议校验，不能通过清洗绕过安全边界。
		if _, public := item["summary"].([]any); public && item["type"] == "reasoning" && strings.HasPrefix(id, "rs_") && responseUUID.MatchString(strings.TrimPrefix(id, "rs_")) && (item["status"] == nil || item["status"] == "completed") {
			delete(item, "id")
			delete(item, "status")
			continue
		}
		switch item["type"] {
		case "message":
			seed := strings.TrimSuffix(id, "_message")
			if seed != id && (strings.HasPrefix(seed, "chatcmpl-") || responseUUID.MatchString(seed)) {
				item["id"] = ResponseTextItemID("msg", seed)
			}
		case "reasoning":
			seed := strings.TrimSuffix(id, "_reasoning")
			if seed != id && (strings.HasPrefix(seed, "chatcmpl-") || responseUUID.MatchString(seed)) {
				delete(item, "id")
				if item["status"] == nil || item["status"] == "completed" {
					delete(item, "status")
				}
			}
		}
	}
}
