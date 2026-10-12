// 本文件统一计时、token 启发式估算、各协议用量提取和分帧合并，保留明确零值与缓存统计。
// 调用关系与边界：数据面用量提取与启发式缺失值补齐；具体参数、返回及副作用见各方法说明。
package usage

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"

	"github.com/sunqirui1987/xhub/internal/util"
)

// TtftMillis 将实测耗时转换为用量日志的可空毫秒数。
// 参数 d 为首响应/首数据或同步完成耗时；返回毫秒指针，非正值返回 nil 表示没有测量。
// 数据面生成 CallNote 时调用；正值不足一毫秒按 1 记录，避免极快响应与未测量混淆。
// 纯转换，无存储副作用；调用方必须保证 d 的语义，函数不会辨别首 token 与整次同步耗时。
func TtftMillis(d time.Duration) *int {
	if d <= 0 {
		return nil
	}
	ms := int(d.Milliseconds())
	if ms < 1 {
		ms = 1
	}
	return &ms
}

// EstimateTokens 用输入文本字节长度估算预算/TPM 预留量，不是分词器，也不是严格数学上界。
// 参数 body 为已解析请求，可为 nil；返回 32 的固定结构开销加可识别文本字节数除以 4。
// 身份限额检查和缺失用量补齐调用；识别 messages 中字符串 content，以及字符串 prompt、input。
// 结构化多模态内容、数组 input 和工具参数不会完整计入；max_tokens 等输出上限不属于输入用量。
// 不修改正文、不访问外部 tokenizer；真实供应商用量存在时应保留真实值，不能用这个启发式覆盖。
func EstimateTokens(body map[string]any) int {
	n := 32
	switch t := body["messages"].(type) {
	case []any:
		for _, m := range t {
			if mm, ok := m.(map[string]any); ok {
				n += len(textOf(mm["content"])) / 4
			}
		}
	}
	if p, ok := body["prompt"].(string); ok {
		n += len(p) / 4
	}
	if p, ok := body["input"].(string); ok {
		n += len(p) / 4
	}
	return n
}

// textOf 安全读取一个动态字段中的字符串。
// 参数 v 为请求字段任意值；返回字符串，非字符串或 nil 返回空串；EstimateTokens 调用。
// 不把对象、数组或数字强行格式化成文本，因此复杂输入不在当前估算范围；纯函数，无错误和副作用。
func textOf(v any) string {
	s, _ := v.(string)
	return s
}

// asInt 将当前支持的 JSON 数字类型转换为整数计数。
// 参数 v 可为 float64、int、int64 或 json.Number；返回整数，未知类型或 Number 解析失败返回零。
// usageCount 调用；float64 直接截断小数，不做范围校验，不能把返回零理解为上游一定报告了零。
// 是否报告字段由 usageCount 的 bool 另行表达；纯转换，不修改用量。
func asInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	case json.Number:
		n, _ := t.Int64()
		return int(n)
	default:
		return 0
	}
}

// CompleteUsage 在上游未报告输入或输出 token 时补入启发式估算，并保留明确报告的零值。
// 参数 usage 为已收集用量，可为 nil；body 为本次输入；streamed 为已捕获 SSE。返回保证非 nil 的用量映射。
// 适配流成功路径调用；优先识别 prompt/input 与 completion/output 别名，只有缺失维度才分别估算。
// 原地修改已有 usage，并保留缓存 token 与扩展明细；输出无可识别文本时可仍缺少 completion_tokens，不强行伪造正值。
// 估算发生时写调试日志以便排查费用来源；捕获截断和结构化输入使估算有局限，不可当作实测账单。
func CompleteUsage(usage map[string]any, body map[string]any, streamed []byte) map[string]any {
	if usage == nil {
		usage = map[string]any{}
	}
	_, promptReported := usageCount(usage, "prompt_tokens", "input_tokens")
	_, completionReported := usageCount(usage, "completion_tokens", "output_tokens")
	// 明确报告的零值也是有效事实，不能用估算覆盖。只有缺失维度才补齐，
	// 并留下诊断提示，方便调用方区分供应商实测与本地启发式估算。
	estimated := false
	if !promptReported {
		if n := EstimateTokens(body); n > 0 {
			usage["prompt_tokens"] = n
			estimated = true
		}
	}
	if !completionReported {
		if n := OutputTokens(streamed); n > 0 {
			usage["completion_tokens"] = n
			estimated = true
		}
	}
	if estimated {
		util.Debug("usage completed by estimate prompt=%v completion=%v", usage["prompt_tokens"], usage["completion_tokens"])
	}
	return usage
}

// OutputTokens 从捕获 SSE 的可识别文本增量粗估输出 token 数。
// 参数 raw 为 SSE/逐行 JSON 字节；返回累计文本字节数除以 4，有文本至少 1，无文本为 0。
// CompleteUsage 仅在缺少实测输出时调用；读取 Chat 首个 choice 的 delta.content 与 Responses 的 output_text.delta。
// 跳过 DONE、空行和非法 JSON，不计工具参数或其他多模态内容；按行解析而非完整多行 SSE，结果只作估算。
// 不修改输入或流响应，捕获不完整时不能据此推断完整供应商用量。
func OutputTokens(raw []byte) int {
	var b strings.Builder
	for _, line := range bytes.Split(raw, []byte("\n")) {
		line = bytes.TrimSpace(bytes.TrimPrefix(bytes.TrimSpace(line), []byte("data:")))
		if len(line) == 0 || bytes.Equal(line, []byte("[DONE]")) {
			continue
		}
		var doc map[string]any
		if json.Unmarshal(line, &doc) != nil {
			continue
		}
		if choices, ok := doc["choices"].([]any); ok && len(choices) > 0 {
			choice, _ := choices[0].(map[string]any)
			delta, _ := choice["delta"].(map[string]any)
			if part, ok := delta["content"].(string); ok {
				b.WriteString(part)
			}
		}
		if doc["type"] == "response.output_text.delta" {
			if part, ok := doc["delta"].(string); ok {
				b.WriteString(part)
			}
		}
	}
	if b.Len() == 0 {
		return 0
	}
	n := b.Len() / 4
	if n < 1 {
		n = 1
	}
	return n
}

// UsageCounts 读取统一输入/输出 token 计数，兼容两组常用供应商字段名。
// 参数 usage 为用量映射，可为 nil；返回 pt、ct，缺失或不可识别数字均为零。
// 指标日志与 BodyUsage 调用；优先 prompt_tokens/completion_tokens，其次 input_tokens/output_tokens。
// 明确零值优先于后续别名；如需区分缺失与零，必须用 usageCount 的报告标志，不能只看这两个返回数字。
func UsageCounts(usage map[string]any) (pt, ct int) {
	pt, _ = usageCount(usage, "prompt_tokens", "input_tokens")
	ct, _ = usageCount(usage, "completion_tokens", "output_tokens")
	return pt, ct
}

// usageCount 按别名优先级读取第一个明确报告的受支持数值字段。
// 参数 usage 为用量，keys 为优先排列的字段名；返回计数与 reported，明确零为 (0, true)，未报告为 (0, false)。
// CompleteUsage、UsageCounts 与 Gemini 归一化调用；非法字段类型跳过并继续检查下一个别名。
// 这种区分防止补齐逻辑把供应商明确的零计费改成估算；仅读取映射，不做负数或范围修正。
func usageCount(usage map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		if value, ok := usage[key]; ok {
			switch value.(type) {
			case float64, int, int64, json.Number:
				return asInt(value), true
			}
		}
	}
	return 0, false
}

// UsageFromDocument 提取普通 JSON 或流事件中的供应商用量，并归一 Gemini 字段。
// 参数 doc 为解析文档；返回直接 usage、response.usage 或新建的 Gemini 用量映射，未报告返回 nil。
// 流解析与原生/适配同步记账调用；Gemini 输出包含 candidatesTokenCount 与 thoughtsTokenCount，缓存输入写入明细。
// 保留字段缺失与明确零的差异；普通包络返回内部映射，Gemini 创建新映射，调用方不要假定所有返回值都深拷贝。
// 不估算、不定价，不从正文内容推断供应商未报告的用量。
func UsageFromDocument(doc map[string]any) map[string]any {
	if usage, ok := doc["usage"].(map[string]any); ok {
		return usage
	}
	if response, ok := doc["response"].(map[string]any); ok {
		if usage, ok := response["usage"].(map[string]any); ok {
			return usage
		}
	}
	metadata, ok := doc["usageMetadata"].(map[string]any)
	if !ok {
		return nil
	}
	usage := map[string]any{}
	if n, present := usageCount(metadata, "promptTokenCount"); present {
		usage["prompt_tokens"] = n
	}
	completion, candidatesPresent := usageCount(metadata, "candidatesTokenCount")
	thoughts, thoughtsPresent := usageCount(metadata, "thoughtsTokenCount")
	// Gemini 把可见输出与思考 token 分开报告；统一输出口径需同时包含两者，
	// 但两项都缺失时仍保留缺失状态，交给上层决定是否估算。
	if candidatesPresent || thoughtsPresent {
		usage["completion_tokens"] = completion + thoughts
	}
	if n, present := usageCount(metadata, "cachedContentTokenCount"); present {
		usage["prompt_tokens_details"] = map[string]any{"cached_tokens": n}
	}
	return usage
}

// MergeUsage 递归合并连续事件中的部分用量，后报告字段覆盖同名旧值。
// 参数 dst 为先前用量，可为 nil；src 为新事件；返回合并后的映射，空 src 直接返回 dst。
// 流解析器调用并递归合并明细；缺失字段保留，明确零可以覆盖旧值。这里是覆盖，不是对每帧累计相加。
// 会原地修改 dst，标量及部分嵌套数据可共享引用；限单请求使用，不可用于无锁并发共享状态。
func MergeUsage(dst, src map[string]any) map[string]any {
	if len(src) == 0 {
		return dst
	}
	if dst == nil {
		dst = map[string]any{}
	}
	for key, value := range src {
		// 流事件通常报告累计快照或部分明细；相加会把重复快照计费多次。
		// 嵌套覆盖则保留前帧未重报的缓存明细，并允许后帧明确零值生效。
		if incoming, ok := value.(map[string]any); ok {
			existing, _ := dst[key].(map[string]any)
			dst[key] = MergeUsage(existing, incoming)
			continue
		}
		dst[key] = value
	}
	return dst
}

// BodyUsage 从同步 JSON 响应提取统一输入和输出 token 计数。
// 参数 raw 为完整响应字节；返回 pt、ct，非法 JSON、缺少用量或未知数字类型均得到零。
// 适配同步与缓存命中指标调用，内部复用 UsageFromDocument 和 UsageCounts 的包络/别名规则。
// 纯解析，不补估算、不修改响应、不访问网络；返回零不能单独证明供应商已报告零用量。
func BodyUsage(raw []byte) (pt, ct int) {
	var doc map[string]any
	if json.Unmarshal(raw, &doc) != nil {
		return 0, 0
	}
	usage := UsageFromDocument(doc)
	return UsageCounts(usage)
}
