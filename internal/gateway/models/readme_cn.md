# models：部署、凭据、端点能力与路由配置管理

[English](readme.md) · [上层架构](../readme_cn.md)

## 职责

部署、凭据、端点能力与路由配置管理。不执行每次用户推理；先看声明校验，再看写入、发现和删除清理。

## Review 顺序

host.go / mount.go → admin.go / validate.go → available.go / discovery.go → groups.go / fallback.go / routing_groups.go → price.go / schedule.go

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

config/provider 声明模型与能力，catalog 提供价格，Host 注入存储、鉴权和探测。

## 验证

同目录测试：admin_test.go, available_test.go, builtin_test.go, discovery_test.go, fallback_test.go, groups_test.go, list_test.go, price_selection_test.go, price_test.go, routing_groups_test.go, schedule_test.go, validate_credential_test.go

仓库根目录执行：go test ./internal/gateway/models -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
