# runtime：运行时快照、冷却、延迟与用量刷新

[English](readme.md) · [上层架构](../readme_cn.md)

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

宿主实现 Runtime 并共享模型、忙碌计数、Redis 和账务依赖。每轮 State 取快照，执行结束记录失败/延迟/用量；后台 Flush 在数据库提交后确认队列。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| State(host) | 返回 router.State 快照 |
| RecordFailure / RecordLatency / RecordUsage | 更新冷却、延迟和用量热状态 |
| Flush / FlushLoop | 同步或周期刷新用量队列 |

### 接入 demo

这是供宿主调用的 Go 接入函数。将代码放在仓库内独立包中可编译；实际执行前须由调用方提供下述真实依赖。

```go
package demo

import (
	"github.com/sunqirui1987/xhub/internal/dataplane/runtime"
	"github.com/sunqirui1987/xhub/internal/router"
)

// RoutingSnapshot 演示每请求运行快照；传入完整宿主，返回调度事实，不发送推理。
func RoutingSnapshot(h runtime.Runtime) router.State {
	return runtime.State(h)
}
```

这是可编译的宿主接入函数；生产实现见 gateway/wire.go，不能用 nil Runtime 代替依赖。

### 错误、状态与副作用

无 Redis 时忙碌计数只在当前进程共享。FlushLoop 没有 context 退出参数，应在进程生命周期中启动一次；刷新失败不确认队列。

## 职责

运行时快照、冷却、延迟与用量刷新。不解析用户协议；刷新只有持久化成功后才能确认，避免丢账单。

## Review 顺序

live.go: Runtime → State → RecordFailure / RecordLatency → Flush / FlushLoop

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

消费方 Runtime 注入模型、并发、Redis、用量存储；gateway/wire.go 实现。

## 验证

同目录测试：live_test.go

仓库根目录执行：go test ./internal/dataplane/runtime -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
