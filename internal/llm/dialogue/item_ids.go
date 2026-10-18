package dialogue

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// ResponseTextItemID 为网关合成的 Responses 正文或公开推理项生成稳定 ID。
// 参数 prefix 是调用方固定的 msg/rs 类型前缀，responseID 是上游响应级 ID；返回类型前缀、xhub 命名空间和十六进制摘要组成的 ID。
// 同步与 SSE 编码器共用；xhub 命名空间标识合成项，空或特殊上游 ID 不直接拼入结果，不修改原生 ID/工具 call_id。
func ResponseTextItemID(prefix, responseID string) string {
	sum := sha256.Sum256([]byte("xhub:responses-text:v1\x00" + prefix + "\x00" + responseID))
	return prefix + "_xhub_" + hex.EncodeToString(sum[:16])
}

// normalizeLegacyResponsesTextIDs 修复旧桥接正文 ID，并移除网关合成的公开推理历史 ID。
// 参数 body 是已复制的 Responses 正文；原地处理完整公开项，无返回值、无外部副作用。
// 公开摘要不是原厂存储引用；仅改成 rs_ 前缀仍会触发 item not found，因此回放时省略合成 ID、保留摘要。
// 原生合法 ID、非字符串 ID、加密项和 item_reference 均不改写；被引用项也保持原样，避免把引用伪造成新归属。
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
			continue
		}
		if !strings.HasPrefix(id, "chatcmpl-") {
			continue
		}
		switch item["type"] {
		case "message":
			if strings.HasSuffix(id, "_message") {
				item["id"] = ResponseTextItemID("msg", strings.TrimSuffix(id, "_message"))
			}
		case "reasoning":
			if strings.HasSuffix(id, "_reasoning") {
				delete(item, "id")
			}
		}
	}
}
