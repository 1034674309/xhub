# health：存活、就绪与控制台启动配置

[English](readme.md) · [上层架构](../readme_cn.md)

## 职责

存活、就绪与控制台启动配置。不依赖 Server、不改模型、不记账；存活不查数据库，就绪失败返回 503。

## Review 顺序

health.go: Pinger → Module → healthLive / healthReady / uiConfig

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

只注入 Ping 和公开版本，PublicOrigin 读取公开地址。

## 验证

同目录测试：health_test.go

仓库根目录执行：go test ./internal/gateway/health -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
