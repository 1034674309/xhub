package diagnostics

import (
	"github.com/sunqirui1987/xhub/internal/util"
)

// TraceHop 在数据面入口记一条日志，只含路径，不含正文和密钥。
// 参数 path：入站 URL 路径。返回：无。
// 调用：Serve 第一行。无单独测试。
func TraceHop(path string) {
	util.Trace("dataplane hop path=%s", path)
}
