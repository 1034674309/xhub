# routing：候选筛选、模型回退与路由诊断

[English](readme.md) · [上层架构](../readme_cn.md)

## 职责

候选筛选、模型回退与路由诊断。不发上游请求、不结算；无候选需区分缺失、暂停、冷却、零流量与能力不兼容。

## Review 顺序

serve.go: DropDisabled → fallback.go → route_diagnostic.go: DiagnoseRoute

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

provider 验证能力，router 提供策略与状态；execution 和路由预览复用诊断。

## 验证

同目录测试：disabled_test.go, fallback_boundary_test.go, fallback_test.go, prefer_test.go, route_diagnostic_test.go

仓库根目录执行：go test ./internal/dataplane/routing -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
