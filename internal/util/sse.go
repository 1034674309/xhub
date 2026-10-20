package util

import (
	"net/http"
	"strings"
)

// PreventSSETransform 禁止代理压缩或重组实时事件，避免小帧直到终态才从 gzip 缓冲区送达客户端。
// 参数 header 为尚未发出的 SSE 响应头；返回值无，标准/原生/通用流在写状态或正文前调用。
// 保留 no-store/private 等既有缓存限制，空值补 no-cache；重复调用不叠加，nil 安全忽略，不改变正文或全站压缩。
func PreventSSETransform(header http.Header) {
	if header == nil {
		return
	}
	values := header.Values("Cache-Control")
	for _, value := range values {
		for _, directive := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(directive), "no-transform") {
				return
			}
		}
	}
	if strings.TrimSpace(strings.Join(values, "")) == "" {
		values = []string{"no-cache"}
	}
	header.Set("Cache-Control", strings.Join(append(values, "no-transform"), ", "))
}
