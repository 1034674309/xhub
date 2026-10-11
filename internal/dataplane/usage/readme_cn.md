# usage：用量归一、缺失统计估算与首字时间

[English](readme.md) · [上层架构](../readme_cn.md)

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
