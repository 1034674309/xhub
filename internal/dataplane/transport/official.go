// 本文件读取服务端部署凭据及拼接合法查询参数，不从模型名或域名猜供应商默认地址。
// 调用关系与边界：原生出站请求载体、正文/URL 处理和 HTTP 转发；具体参数、返回及副作用见各方法说明。
package transport

import (
	"net/http"

	"github.com/sunqirui1987/xhub/internal/config"
	dpdiagnostics "github.com/sunqirui1987/xhub/internal/dataplane/diagnostics"
)

// BypassAuth 读取实际部署明确配置的供应商根地址与 API 密钥。
// 参数 dep 为已附加凭据的部署；返回去末尾斜杠的根地址及密钥；原生执行发送前调用。
// 缺失值返回空串，执行器负责拒绝；不猜默认域名、不附加凭据、不修改部署，返回密钥不得写入日志。
func BypassAuth(dep config.ModelEntry) (string, string) {
	base := dpdiagnostics.TrimBase(dep.ParamString("api_base", ""))
	return base, dep.ParamString("api_key", "")
}

// WithQuery 将合法入站查询参数加入已构造上游 URL，同时移除调用方查询凭据。
// 参数 url 为根地址加登记路径，r 为入站请求；返回附有编码后查询串的地址，无查询时原样返回。
// 原生创建和查询调用；过滤 key、api_key、access_token，保留分页、任务等其他字段与多值语义。
// 重新编码可能改变参数顺序，不能用于要求原始查询签名字节不变的协议；不改 r、不访问网络。
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
