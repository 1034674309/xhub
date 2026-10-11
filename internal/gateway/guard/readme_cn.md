# guard：护栏配置、规则与外部执行器

[English](readme.md) · [上层架构](../readme_cn.md)

## 职责

护栏配置、规则与外部执行器。不路由模型、不计价；外部错误、秘密脱敏、拦截需一起 review。

## Review 顺序

host.go / mount.go → manage.go → guard.go / runner.go → custom_http.go / external.go

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

Host、存储、内置规则和外部执行器组合，gateway 钩子执行结果。

## 验证

同目录测试：custom_test.go, engine_test.go, external_test.go, guard_regression_test.go, guard_test.go, primitives_test.go

仓库根目录执行：go test ./internal/gateway/guard -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
