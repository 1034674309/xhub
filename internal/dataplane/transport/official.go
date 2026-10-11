package transport

import (
	"net/http"

	"github.com/sunqirui1987/xhub/internal/config"
	dpdiagnostics "github.com/sunqirui1987/xhub/internal/dataplane/diagnostics"
)

// BypassAuth 读取显式配置的上游根地址和密钥，不猜测供应商默认值。
// 参数 dep 为已附加凭据的部署；返回根地址、密钥，空值由执行器拒绝。
// 原生创建和查询调用此函数；不修改部署，无外部调用。
func BypassAuth(dep config.ModelEntry) (string, string) {
	base := dpdiagnostics.TrimBase(dep.ParamString("api_base", ""))
	return base, dep.ParamString("api_key", "")
}

// WithQuery 把入站查询串接到上游 URL 后面。没有查询串时原样返回。
// 参数 url：根地址加上游路径。r：入站请求。
// 返回：可能带 ? 的完整 URL。
// 调用：创建和查询。测试：逻辑测试断言 page_size 和 taskId 被保留。
func WithQuery(url string, r *http.Request) string {
	if r.URL.RawQuery == "" {
		return url
	}
	query := r.URL.Query()
	// 网关凭据不得进入上游 query；供应商认证始终由注册的头注入。
	query.Del("key")
	query.Del("api_key")
	query.Del("access_token")
	if query.Encode() == "" {
		return url
	}
	return url + "?" + query.Encode()
}
