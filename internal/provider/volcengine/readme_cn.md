# 火山引擎方舟 Seedance 注册

[English](readme.md)

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

启动入口空白导入本包触发 init 登记，然后用 registry.Match 取得固定动作。管理员仍需创建部署并明确绑定 transport/endpoint_types、地址和凭据，原生数据面负责真正发送。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| 空白导入 volcengine | 触发 init；没有运行时构造函数 |
| registry.Match(method, path) | 取得传输、动作和路径参数 |

### 接入 demo

将下面代码保存为仓库内的 `demo/main.go`，在仓库根目录执行 `go run ./demo`；运行后删除本次创建的 demo 目录。

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/provider/registry"
	_ "github.com/sunqirui1987/xhub/internal/provider/volcengine"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	hit, ok := registry.Match("POST", "/api/v3/contents/generations/tasks")
	if !ok {
		panic("registered action not found")
	}
	fmt.Println(hit.Transport.ID, hit.Action.Name)
}
```

输出 ark_contents_generation create。示例只验证协议登记，不需要账号、不发送任务。

### 错误、状态与副作用

Ark 使用 Bearer，包含创建、列表和查询白名单；创建与轮询需固定同一部署，成功实测结果才结算。 导入不会连接服务、创建凭据或证明供应商在线；异步任务费用必须由数据面去重。

## 说明

本包声明 VolcEngine supplier 并登记方舟原生异步内容生成。管理员仍需提供 base URL、Bearer 凭据和部署价格；两个 Seedance 模型是目录快捷项。

### 依赖与测试

本包使用 provider 注册表与共享 Seedance 计量。测试验证 supplier/模型、前缀、路径和绑定，不发真实请求。

## 对外接口

本包无导出 API 或 handler。init 使 provider.Match 识别 POST /api/v3/contents/generations/tasks、GET 同路径列表、GET /api/v3/contents/generations/tasks/{id}；原生 dataplane 负责鉴权、固定与结算。

## 内部核心实现

ark_contents_generation 使用 bypass:ark-video、ark 协议、video family、Bearer、model 字段、id 任务字段，并去掉 volcengine/。创建、列表、查询是独立白名单 action。SeedanceBilling 只接受明确成功终态 usage。存储模型 ID 映射到方舟官方名称，价格由部署配置。

## 源码职责与入口

- [seedance.go](seedance.go)：supplier、transport、三个 action、两个模型绑定、鉴权/前缀/任务元数据和计量。

验证命令：go test ./internal/provider/volcengine ./internal/provider -count=1。真实账号、额度、字段和价格属于外部边界。
