# family：目录资源接口与租户隔离

[English](readme.md) · [上层架构](../readme_cn.md)

## 职责

目录资源接口与租户隔离。专门业务接口应由专门模块处理；先看资源隔离再看通用输出。

## Review 顺序

host.go → mount.go → handlers.go

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

Host 注入身份、作用域存储和目录元数据；gateway/catalog.go 注册。

## 验证

同目录测试：pagination_test.go, resource_isolation_test.go

仓库根目录执行：go test ./internal/gateway/family -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
