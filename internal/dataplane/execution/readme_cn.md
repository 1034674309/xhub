# execution：适配与原生请求的生命周期编排

[English](readme.md) · [上层架构](../readme_cn.md)

## 职责

适配与原生请求的生命周期编排。不做控制台 CRUD，不直接访问 PostgreSQL 或 Redis。重试、失败计费与成功终态需一起审核。

## Review 顺序

serve.go: Serve → official.go: ServeBypass

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

contract 注入身份、配额、凭据、HTTP、记账；routing、dialogue、transport、stream、usage、diagnostics 执行专门职责。

## 验证

同目录测试：bypass_logic_test.go, cache_scope_test.go, capability_test.go, endpoint_redesign_test.go, failure_log_test.go, fal_test.go, fallback_boundary_test.go, fallback_test.go, native_bypass_test.go, official_settlement_test.go, official_template_test.go, qiniu_live_test.go, route_diagnostic_test.go, stream_failure_regression_test.go, task_status_test.go, usage_stream_test.go

仓库根目录执行：go test ./internal/dataplane/execution -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
