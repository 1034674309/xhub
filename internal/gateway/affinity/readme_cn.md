# affinity：租户隔离的会话、响应续接和任务归属

[English](readme.md) · [上层架构](../readme_cn.md)

## 职责

租户隔离的会话、响应续接和任务归属。不选择路由策略、不计算价格；响应上下文按调用方、模型、入口隔离，失败不能发布成功续接。

## Review 顺序

affinity.go: New / State → PlanRoute → CommitRoute → PinOfficial / OfficialContext

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

State 独立持有锁、内存、最小 Redis 接口；gateway 组合，execution 消费 contract。

## 验证

同目录测试：affinity_pin_test.go, affinity_test.go, responses_context_test.go

仓库根目录执行：go test ./internal/gateway/affinity -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
