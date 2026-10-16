// 本文件定义编码器交给执行器的出站请求载体，本文件不拨号或推断供应商。
// 调用关系与边界：原生出站请求载体、正文/URL 处理和 HTTP 转发；具体参数、返回及副作用见各方法说明。
package transport

import (
	"net/http"
)

// Upstream 保存编码器按显式执行配置构造的单次出站请求。
// dialogue.BuildDialogue/BuildRegisteredOperation 产生，execution.Serve 消费；只持有 URL、服务端鉴权头和正文，不执行 HTTP。
// 目标由注册路径与部署地址组成，客户端不能用未知供应商名或任意地址改变执行协议。
type Upstream struct {
	// 部署根地址加登记路径及必要查询参数，不能由客户端任意指定。
	URL string
	// 编码器产生的服务端鉴权与协议头；含秘密，不应完整写日志。
	Header http.Header
	// 编码后的出站正文，与入站或护栏前的原始正文分别保存。
	Body []byte
	// 管理员允许的能力降级，执行器写入响应头与请求诊断，不参与鉴权。
	DroppedCapabilities []string
}
