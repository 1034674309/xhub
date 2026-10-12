// 本文件提供安全错误投影、地址主机投影及 token/延迟性能日志，不参与路由或扣费。
// 调用关系与边界：出站事实观察和安全进程日志；具体参数、返回及副作用见各方法说明。
package diagnostics

import (
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/sunqirui1987/xhub/internal/util"
)

// secretInErr 只匹配常见 sk- 密钥和 Bearer 内容；SafeErr 使用，不是任意秘密的通用识别器。
var secretInErr = regexp.MustCompile(`sk-[A-Za-z0-9_\-]+|(?i)bearer\s+\S+`)

// SafeErr 对常见上游错误中的敏感片段做日志脱敏。
// 参数 err 为网络、编码等错误；返回可记录文本，nil 返回空串；执行器的错误日志与失败说明调用。
// 把 HTTP URL 简化为主机名，并替换匹配的 sk- 密钥和 Bearer 内容，减少路径/查询/凭据泄露。
// 这是针对已知模式的过滤，不是所有秘密的通用清洗器；不能把任意供应商正文都视为可安全公开。
// 纯文本处理，不修改原错误或外部状态。
func SafeErr(err error) string {
	if err == nil {
		return ""
	}
	s := secretInErr.ReplaceAllString(err.Error(), "***")
	return regexp.MustCompile(`https?://[^\s"'<>]+`).ReplaceAllStringFunc(s, func(raw string) string {
		host := BaseHost(strings.TrimRight(raw, `",)`))
		if host == "" {
			return "***"
		}
		return host
	})
}

// BaseHost 提取根地址中的主机部分，用于安全定位上游连接。
// 参数 apiBase 为完整 URL；返回解析得到的主机，非法地址或无主机返回空串。
// SafeErr 和执行器日志调用；用户信息、路径和查询不保留，不发送网络探测或修正地址。
func BaseHost(apiBase string) string {
	u, err := url.Parse(apiBase)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Hostname()
}

// TrimBase 去掉根地址末尾斜杠，供登记路径拼接使用。
// 参数 s 为原始根地址；返回清理后的字符串，空值保持空值；请求构造和原生凭据读取调用。
// 仅处理末尾字符，不校验协议、主机或授权范围，也不补供应商默认地址；无副作用。
func TrimBase(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

// LogMetrics 输出一次调用的 token、首数据耗时、总耗时及生成速率摘要。
// 参数 path、model 为入口和公开模型；cacheHit 表示缓存命中；pt、ct 为输入/输出 token；ttft、elapsed 为实测耗时。返回值无。
// Serve 的缓存、流式和同步成功路径调用；缓存命中不计算生成速率，非正生成窗口使用可用总耗时。
// 速率是观测指标而非费用依据；副作用为一条进程日志，不修改账单或推断缺失用量。
func LogMetrics(path, model string, cacheHit bool, pt, ct int, ttft, elapsed time.Duration) {
	window := elapsed - ttft
	if window <= 0 {
		window = elapsed
	}
	perSec := 0.0
	if !cacheHit {
		perSec = tokensPerSecond(ct, window)
	}
	util.Info("process path=%s step=metrics model=%s cache_hit=%t ttft=%s prompt_tokens=%d completion_tokens=%d total_tokens=%d tokens_per_s=%.2f", path, model, cacheHit, ttft, pt, ct, pt+ct, perSec)
}

// tokensPerSecond 根据输出 token 与生成窗口计算每秒速率。
// 参数 completion 为完成 token，window 为时间窗口；返回浮点速率，任一非正时返回零。
// LogMetrics 调用；避免零除产生无穷值，纯计算，不修正调用方传入的计时语义。
func tokensPerSecond(completion int, window time.Duration) float64 {
	if completion <= 0 || window <= 0 {
		return 0
	}
	return float64(completion) / window.Seconds()
}
