# diagnostics：上游原始事实与安全诊断日志

[English](readme.md) · [上层架构](../readme_cn.md)

## 职责

上游原始事实与安全诊断日志。不计算价格，不保存生成内容；路由日志不得泄漏密钥。

## Review 顺序

upstream_diagnostics.go → log.go → host.go: TraceHop

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

execution、transport 采集事实，gateway/requestlog 脱敏并序列化。

## 验证

同目录测试：upstream_diagnostics_test.go

仓库根目录执行：go test ./internal/dataplane/diagnostics -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
