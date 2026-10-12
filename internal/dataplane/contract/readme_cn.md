# contract：执行器能力接口和调用事实

[English](readme.md) · [上层架构](../readme_cn.md)

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

宿主实现 Adapted 或 Bypass 的全部方法，再交给 execution；RequestHost 提供公共能力。RoutePlan 是单请求私有计划，成功后提交实际 Target 和完整历史。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| RequestHost / Adapted / Bypass | 执行器向宿主请求依赖的接口 |
| RoutePlan / RouteTarget / RouteStep | 请求计划、稳定目标和回退路径 |
| CallNote / SpendLog | 结算及日志事实契约 |

### 接入 demo

将下面代码保存为仓库内的 `demo/main.go`，在仓库根目录执行 `go run ./demo`；运行后删除本次创建的 demo 目录。

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/dataplane/contract"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	plan := contract.RoutePlan{Alias: "demo-model", Target: contract.RouteTarget{DeploymentID: "demo-deployment", Model: "demo-model", Pool: "demo-model"}}
	if plan.Err != nil {
		panic(plan.Err)
	}
	fmt.Println(plan.Alias, plan.Target.DeploymentID, plan.Required)
}
```

输出 demo-model demo-deployment false；只演示值类型，不代表该候选已获得授权或执行成功。

### 错误、状态与副作用

Required 固定目标不能当可选偏好；缓存目标不携带权限。接口包不连接服务、不转发 HTTP，也不自动检查实现是否符合业务约束。

## 职责

执行器能力接口和调用事实。不转发 HTTP、不选路由、不写数据库。

## Review 顺序

host.go: RequestHost → Adapted / Bypass → SpendLog, RoutePlan, CallNote

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

execution 消费，gateway 实现；auth、config、router、prefs、provider 提供值类型。

## 验证

同目录测试：本包仅接口声明，通过组合编译和后台回归验证。

仓库根目录执行：go test ./internal/dataplane/contract -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
