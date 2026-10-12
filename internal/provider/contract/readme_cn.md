# contract

[English](readme.md)

共享数据契约.

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

新增协议时构造 Transport/Action/AuthConfig 并交给 registry 注册；数据面使用 Hit 匹配结果。TaskContext 只保存创建时最小计价事实，成功终态再提取实际用量。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| Transport / Action / Hit | 协议、白名单动作与路径捕获 |
| ReadTaskID(doc, path) | 按点分路径读取任务 ID |
| OfficialID(prefix, stored) | 只剥离一层供应商前缀 |

### 接入 demo

将下面代码保存为仓库内的 `demo/main.go`，在仓库根目录执行 `go run ./demo`；运行后删除本次创建的 demo 目录。

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/provider/contract"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	doc := map[string]any{"data": map[string]any{"task_id": "demo-task"}}
	fmt.Println(contract.ReadTaskID(doc, "data.task_id"))
	fmt.Println(contract.OfficialID("demo", "demo/org/model"))
}
```

输出 demo-task 和 org/model；纯函数不注册协议、不发送请求。

### 错误、状态与副作用

Expand 只是文本替换，不做 URL 编码；实际出站路径应使用 transport 的构造器。任务上下文不是实测用量，不保存凭据或提示词。

## 源码与验证

代码入口：[type.go](type.go)

单元测试与源代码放在一起：[match_test.go](match_test.go)

go test ./internal/provider/contract/...

真实后台回归在 cmd/regression，浏览器流程在 frontend/e2e。离线测试使用本地 HTTP 上游，不证明生产凭据或外部厂商服务可用性。
