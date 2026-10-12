// 本文件提供数据面入口路径追踪，便于确认请求是否进入执行器。
// 调用关系与边界：出站事实观察和安全进程日志；具体参数、返回及副作用见各方法说明。
package diagnostics

import (
	"github.com/sunqirui1987/xhub/internal/util"
)

// TraceHop 在数据面入口记录路径级跟踪信息。
// 参数 path 为入站 URL 路径；返回值无；Serve 在进入处理链时调用。
// 只输出路径，不附带请求正文、查询参数或凭据；副作用是进程跟踪日志，不影响路由或用量。
func TraceHop(path string) {
	util.Trace("dataplane hop path=%s", path)
}
