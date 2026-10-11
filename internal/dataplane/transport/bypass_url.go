package transport

import (
	"net/url"
	"strings"

	providercontract "github.com/sunqirui1987/xhub/internal/provider/contract"
)

// bypassURL 将供应商根地址与协议路径组合，避免根路径重复。
// 参数 base、path（string）：供应商地址和已登记上游路径。返回 string：完整请求地址。
// 调用：bypassDeploymentURL。测试：native_bypass_test.go。
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

// BypassDeploymentURL 按供应商协议前缀生成当前操作的上游 URL。
// 参数 base：凭据根地址；hit：匹配的已登记操作与路径参数。
// 返回 string：只由已登记路径及编码后的任务参数构成的地址。
// 调用：原生创建、查询。测试：TestNativeSupplierURLs。
func BypassDeploymentURL(base string, hit providercontract.Hit) string {
	names := make(map[string]string, len(hit.Names))
	for k, v := range hit.Names {
		names[k] = url.PathEscape(v)
	}
	path := providercontract.Expand(hit.Action.UpstreamPath, names)
	return bypassURL(base, path)
}
