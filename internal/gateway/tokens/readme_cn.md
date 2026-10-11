# tokens：本地 token 估算、参数发现与转换记录

[English](readme.md) · [上层架构](../readme_cn.md)

## 职责

本地 token 估算、参数发现与转换记录。不发供应商推理，估算不等于账单实测；存储失败应可观察。

## Review 顺序

tokens.go: Host / Store → Module → tokenCounter / supportedOpenAIParams / transformRequest

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

Host 注入鉴权和模型快照，Store 提供 KV，llm/estimate 计数。

## 验证

同目录测试：tokens_test.go

仓库根目录执行：go test ./internal/gateway/tokens -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
