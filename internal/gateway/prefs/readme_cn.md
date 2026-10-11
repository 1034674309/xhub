# prefs：偏好与路由设置的读取、合并和保存

[English](readme.md) · [上层架构](../readme_cn.md)

## 职责

偏好与路由设置的读取、合并和保存。不执行路由；缺省与显式零不同，合并顺序影响租户生效设置。

## Review 顺序

host.go / mount.go → settings.go / page.go → route_settings.go / merge.go

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

Host 注入存储、鉴权及 reload，router 定义策略值。

## 验证

同目录测试：allocation_test.go, fallback_test.go, route_settings_test.go, routing_groups_test.go

仓库根目录执行：go test ./internal/gateway/prefs -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
