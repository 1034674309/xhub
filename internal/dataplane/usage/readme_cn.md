# usage：用量归一、缺失统计估算与首字时间

[English](readme.md) · [上层架构](../readme_cn.md)

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

把供应商响应交给 UsageFromDocument/UsageCounts，缺少统计时才 CompleteUsage。保持明确零与缺失的差异，再交 catalog 规范化和计价。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| UsageCounts(usage) | 返回输入/输出 token 数 |
| CompleteUsage / MergeUsage | 补充或合并映射，可能原地修改 |
| TtftMillis(duration) | 返回可空毫秒数，正时长最小 1ms |

### 接入 demo

将下面代码保存为仓库内的 `demo/main.go`，在仓库根目录执行 `go run ./demo`；运行后删除本次创建的 demo 目录。

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/dataplane/usage"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	input, output := usage.UsageCounts(map[string]any{"input_tokens": 100, "output_tokens": 20})
	fmt.Println(input, output)
}
```

输出 100 20；不决定单价，也不写账务。

### 错误、状态与副作用

估算 token 不是保证上界，也不是实测。MergeUsage 修改目标映射；需要保留输入的调用方先复制。

## 职责

用量归一、缺失统计估算与首字时间。显式零与缺失必须区分；估算不冒充供应商实测，不制定单价。

## Review 顺序

usage.go: UsageCounts / UsageFromDocument / MergeUsage → CompleteUsage / TtftMillis

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

stream、dialogue、execution 消费纯函数；catalog 与 gateway/billing 管理价格。

## 验证

同目录测试：metrics_test.go, usage_stream_test.go

仓库根目录执行：go test ./internal/dataplane/usage -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
