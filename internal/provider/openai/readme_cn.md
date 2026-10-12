# openai

[English](readme.md)

OpenAI 原生协议注册.

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

启动入口空白导入本包触发 init 登记，然后用 registry.Match 取得固定动作。管理员仍需创建部署并明确绑定 transport/endpoint_types、地址和凭据，原生数据面负责真正发送。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| 空白导入 openai | 触发 init；没有运行时构造函数 |
| registry.Match(method, path) | 取得传输、动作和路径参数 |

### 接入 demo

将下面代码保存为仓库内的 `demo/main.go`，在仓库根目录执行 `go run ./demo`；运行后删除本次创建的 demo 目录。

```go
package main

import (
	"fmt"
	_ "github.com/sunqirui1987/xhub/internal/provider/openai"
	"github.com/sunqirui1987/xhub/internal/provider/registry"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	hit, ok := registry.Match("POST", "/bypass/openai/v1/responses")
	if !ok {
		panic("registered action not found")
	}
	fmt.Println(hit.Transport.ID, hit.Action.Name)
}
```

输出 bypass_openai_responses create。示例只验证协议登记，不需要账号、不发送任务。

### 错误、状态与副作用

部署凭据写入 Authorization: Bearer；视频成功实测秒数才结算。 导入不会连接服务、创建凭据或证明供应商在线；异步任务费用必须由数据面去重。

## 源码与验证

代码入口：[native.go](native.go)

单元测试与源代码放在一起：[native_test.go](native_test.go), [registration_test.go](registration_test.go)

go test ./internal/provider/openai/...

真实后台回归在 cmd/regression，浏览器流程在 frontend/e2e。离线测试使用本地 HTTP 上游，不证明生产凭据或外部厂商服务可用性。
