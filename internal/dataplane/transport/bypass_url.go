// 本文件按登记路径拼接部署 URL，保留编码后的模型/任务参数并避免重复根路径。
// 调用关系与边界：原生出站请求载体、正文/URL 处理和 HTTP 转发；具体参数、返回及副作用见各方法说明。
package transport

import (
	"net/url"
	"strings"

	providercontract "github.com/sunqirui1987/xhub/internal/provider/contract"
)

// bypassURL 将配置根地址与登记操作路径拼成完整上游 URL。
// 参数 base 为服务端根地址，path 为登记路径；返回组合地址；BypassDeploymentURL 调用。
// 避免根路径和 v1 前缀重复，处理已经转义的任务参数，防止模型/任务里的斜杠被解释成新路由层级。
// 纯构造，不校验网络可达性或补供应商默认根地址；地址与 path 的信任边界由部署和注册目录保证。
func bypassURL(base, path string) string {
	u, err := url.Parse(base)
	if err != nil {
		return base + path
	}
	root := strings.TrimRight(u.Path, "/")
	if root != "" && (path == root || strings.HasPrefix(path, root+"/")) {
		u.Path = path
	} else if strings.HasSuffix(root, "/v1") && strings.HasPrefix(path, "/v1/") {
		u.Path = root + strings.TrimPrefix(path, "/v1")
	} else {
		u.Path = root + path
	}
	// 保留注册参数的转义，防止模型或任务 ID 中的斜杠变为路径分隔符。
	encoded := u.Path
	if decoded, err := url.PathUnescape(encoded); err == nil {
		u.Path = decoded
		u.RawPath = encoded
	}
	return u.String()
}

// BypassDeploymentURL 按已匹配协议操作构造当前部署的请求地址。
// 参数 base 为附加凭据后的根地址，hit 为登记操作及路径参数；返回对应 URL；原生创建/后续请求调用。
// 以显式传输规则处理协议前缀及路径参数，使用编码后路径/RawPath 保留参数边界，不从主机名猜供应商。
// 不附带调用方查询凭据，不发网络请求；其他合法查询由 WithQuery 单独接入。
func BypassDeploymentURL(base string, hit providercontract.Hit) string {
	names := make(map[string]string, len(hit.Names))
	for k, v := range hit.Names {
		names[k] = url.PathEscape(v)
	}
	path := providercontract.Expand(hit.Action.UpstreamPath, names)
	return bypassURL(base, path)
}
