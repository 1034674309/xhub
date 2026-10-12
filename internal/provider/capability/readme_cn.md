# capability

[English](readme.md)

公开端点与协议匹配.

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

管理端用 EndpointTypes 展示可选入口，运行时通过 EndpointForOp 与 CompatibleEndpoint 检查声明。能力标识描述公开入口，transport 标识描述上游执行，两者不能互相猜测。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| EndpointTypes / KnownEndpoint | 公开端点目录与存在性判断 |
| CompatibleEndpoint(endpoint, transport) | 验证入口和执行传输兼容 |
| DialogueProtocol(protocol) | 判断统一对话编解码支持 |

### 接入 demo

将下面代码保存为仓库内的 `demo/main.go`，在仓库根目录执行 `go run ./demo`；运行后删除本次创建的 demo 目录。

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/provider/capability"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	fmt.Println(capability.KnownEndpoint("responses"))
	fmt.Println(capability.DialogueProtocol("anthropic-messages"))
	fmt.Println(capability.KnownEndpoint("unknown-demo"))
}
```

输出 true、true、false；目录存在不表示任何部署都能执行。

### 错误、状态与副作用

不要通过 URL、供应商名称或 token 价格猜能力。新增协议需要同时登记入口、执行方式，并补充兼容与失败路径测试。

## 源码与验证

代码入口：[capability.go](capability.go), [endpoints.go](endpoints.go), [execution.go](execution.go)

单元测试与源代码放在一起：[capability_test.go](capability_test.go)

go test ./internal/provider/capability/...

真实后台回归在 cmd/regression，浏览器流程在 frontend/e2e。离线测试使用本地 HTTP 上游，不证明生产凭据或外部厂商服务可用性。
