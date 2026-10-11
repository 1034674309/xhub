# usage：报表、活动、护栏统计与日志读取

[English](readme.md) · [上层架构](../readme_cn.md)

## 职责

报表、活动、护栏统计与日志读取。不产生账单、不调供应商；先看租户读权限，再看筛选、分页与金额。

## Review 顺序

host.go / mount.go → reports.go → activity.go / entity_activity.go → guardrails.go

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

Host 提供作用域身份和存储，IAM/store 查询持久化数据。

## 验证

同目录测试：activity_test.go, cost_breakdown_test.go, guardrails_test.go, health_connection_test.go, log_guardrail_test.go, reports_test.go

仓库根目录执行：go test ./internal/gateway/usage -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
