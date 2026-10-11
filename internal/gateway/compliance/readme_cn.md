# compliance：请求日志合规检查接口

[English](readme.md) · [上层架构](../readme_cn.md)

## 职责

请求日志合规检查接口。只计算提交记录的规则结果，不执行护栏、不持久化，也不替代法律评估。

## Review 顺序

compliance.go: Host / Module → writeCompliance → euAIActChecks / gdprChecks

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

最小 Host 仅管理鉴权，util 挂路由与输出。

## 验证

同目录测试：compliance_test.go

仓库根目录执行：go test ./internal/gateway/compliance -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
