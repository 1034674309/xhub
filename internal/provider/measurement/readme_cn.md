# measurement

[English](readme.md)

响应/SSE 用量和任务计量事实.

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

传输注册时注入 NativeUsage 或任务 Billing 回调。执行器把原始响应与可信创建上下文交给回调，结果交 catalog 计价；回调不算金额、不写账单。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| NativeUsage(protocol) | 返回响应/请求事实提取函数 |
| NativeStreamUsage(protocol, raw, request) | 提取完整 SSE 事件用量 |
| SeedanceBilling / FalBilling | 构造任务上下文与成功终态提取器 |

### 接入 demo

将下面代码保存为仓库内的 `demo/main.go`，在仓库根目录执行 `go run ./demo`；运行后删除本次创建的 demo 目录。

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/provider/measurement"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	extract := measurement.NativeUsage("openai-chat")
	usage := extract(map[string]any{"usage": map[string]any{"prompt_tokens": 100, "completion_tokens": 20}}, nil)
	fmt.Println(usage["prompt_tokens"], usage["completion_tokens"])
}
```

输出 100 20；离线夹具演示事实提取。

### 错误、状态与副作用

缺少用量或媒体规格时保留 pricing_blocked，不能猜免费或请求量就是输出量。失败/未完成终态不能据创建请求提前计费。

## 源码与验证

代码入口：[fal_billing.go](fal_billing.go), [native_usage.go](native_usage.go), [seedance_billing.go](seedance_billing.go)

单元测试与源代码放在一起：[bindings_test.go](bindings_test.go), [google_usage_test.go](google_usage_test.go), [native_usage_test.go](native_usage_test.go)

go test ./internal/provider/measurement/...

真实后台回归在 cmd/regression，浏览器流程在 frontend/e2e。离线测试使用本地 HTTP 上游，不证明生产凭据或外部厂商服务可用性。
