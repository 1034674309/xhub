# identity：用户、团队成员与管理权限

[English](readme.md) · [上层架构](../readme_cn.md)

## 职责

用户、团队成员与管理权限。不转发推理；先看鉴权，再看成员与模板，防止越权写入。

## Review 顺序

gate.go → mount.go → handlers.go / members.go / route_template.go

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

auth、IAM 提供身份和成员关系；模块边界注入存储与管理。

## 验证

同目录测试：pagination_test.go, route_template_error_test.go

仓库根目录执行：go test ./internal/gateway/identity -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
