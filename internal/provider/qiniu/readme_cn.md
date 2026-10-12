# 七牛 Fal 与内容生成模型目录

[English](readme.md)

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

启动入口空白导入本包触发 init 登记，然后用 registry.Match 取得固定动作。管理员仍需创建部署并明确绑定 transport/endpoint_types、地址和凭据，原生数据面负责真正发送。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| 空白导入 qiniu | 触发 init；没有运行时构造函数 |
| registry.Match(method, path) | 取得传输、动作和路径参数 |

### 接入 demo

将下面代码保存为仓库内的 `demo/main.go`，在仓库根目录执行 `go run ./demo`；运行后删除本次创建的 demo 目录。

```go
package main

import (
	"fmt"
	_ "github.com/sunqirui1987/xhub/internal/provider/qiniu"
	"github.com/sunqirui1987/xhub/internal/provider/registry"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	hit, ok := registry.Match("POST", "/v3/contents/generations/tasks")
	if !ok {
		panic("registered action not found")
	}
	fmt.Println(hit.Transport.ID, hit.Action.Name)
}
```

输出 qiniu_contents_generation create。示例只验证协议登记，不需要账号、不发送任务。

### 错误、状态与副作用

内容生成使用 Bearer，Fal 队列使用 Key；内容生成只登记创建和查询，不支持 list。 导入不会连接服务、创建凭据或证明供应商在线；异步任务费用必须由数据面去重。

## 说明

本包登记七牛兼容 Fal 队列 transport 和 /v3 内容生成 transport。连接由管理员通过 Custom/OpenAI 兼容部署提供；注册过程不创建账号，也不读取凭据。模型条目分别保存上游 ID、定价 ID、来源、transport ID 与计量维度。

### 依赖与测试

本包使用 provider 注册表和共享 Fal/Seedance 计量。测试覆盖精确路径、鉴权、前缀、实测单位、成功边界、变体，以及上下文不足时阻止自动定价。

## 对外接口

本包无导出函数和本地路由。Fal 提供精确 POST /queue/{模型路径}、GET /queue/{组}/requests/{request_id}、GET .../status。内容生成提供 POST /v3/contents/generations/tasks 和 GET /v3/contents/generations/tasks/{id}，没有 list。provider.Match 命中后交给原生 dataplane。

## 内部核心实现

registerFal 为每个队列族创建 transport、两个共享查询 action 和每个具体模型的 create action。它使用 Authorization: Key、request_id，去掉 qiniu/，改写队列 URL，并按分辨率、音频、变体绑定 FalBilling。init 明确枚举 Seedance/Dreamina、Kling、Vidu、Veo、MiniMax，通配符不能扩大白名单。

内容 transport 使用 Bearer、id、前缀移除和 SeedanceBilling；四个 bytedance Seedance 模型分别保存市场定价 ID 与来源。后续结算要求明确成功终态和正数实测 usage。

## 源码职责与入口

- [fal.go](fal.go)：falEndpoint、registerFal、具体队列 action/模型、目录、Key 鉴权、URL 改写与 Fal 计量。
- [seedance.go](seedance.go)：qiniu_contents_generation 创建/查询、四个模型、Bearer、前缀和 Seedance 计量。

验证命令：go test ./internal/provider/qiniu ./internal/provider -count=1。可用性、额度、时长与价格时效属于外部边界。
