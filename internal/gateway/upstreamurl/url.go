// Package upstreamurl 构造入口展示的上游路径，不处理请求正文或鉴权。
package upstreamurl

import (
	"net/url"
	"path"
	"strings"
	"sync"

	"github.com/sunqirui1987/xhub/internal/util"
)

var logTraceOncePassthrough sync.Once

// PassthroughURL 将 endpoint 接到显式 base 地址，供 gateway/ingress 展示透传契约。
// 参数 base 为连接地址、endpoint 为注册路径、provider 为协议标识；返回拼接地址，解析失败保留 base。
// 仅 OpenAI 官方域名补齐 v1；路径越过已有根前缀时退回根路径，无网络调用或默认地址推断。
func PassthroughURL(base, endpoint, provider string) string {
	logTraceOncePassthrough.Do(func() { util.Trace("enter upstreamurl.PassthroughURL") })

	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	if endpoint != "" && !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	u.Path = joinBaseAndEndpoint(u.Path, endpoint)
	u.RawPath = ""
	out := u.String()
	if (provider == "openai") && !strings.Contains(out, "/v1/") {
		out = strings.Replace(out, "api.openai.com/", "api.openai.com/v1/", 1)
	}
	return out
}

// joinBaseAndEndpoint joins an upstream base and an endpoint path without a double slash and without dropping a prefix already on the base.
// 参数 basePath（string）：拼接根地址和端点使用的根地址路径。空串表示调用方没有提供这项；endpointPath（string）：拼接根地址和端点使用的端点路径。空串表示调用方没有提供这项。
// 返回 string（string）：接好的路径。不会出现双斜杠，也不会丢掉 base 上已有的前缀。endpoint 原来以斜杠结尾时保留。
// 调用：仅在 passthrough.go 内使用
// 测试：无直接单测
func joinBaseAndEndpoint(basePath, endpointPath string) string {
	trailing := strings.HasSuffix(endpointPath, "/")
	if basePath == "" || basePath == "/" {
		normalized := path.Clean("/" + strings.TrimPrefix(endpointPath, "/"))
		if trailing && normalized != "/" {
			normalized += "/"
		}
		return normalized
	}
	basePath = strings.TrimRight(basePath, "/")
	combined := path.Clean(basePath + "/" + strings.TrimPrefix(endpointPath, "/"))
	if combined != basePath && !strings.HasPrefix(combined, basePath+"/") {
		return basePath + "/"
	}
	if trailing && !strings.HasSuffix(combined, "/") {
		combined += "/"
	}
	return combined
}
