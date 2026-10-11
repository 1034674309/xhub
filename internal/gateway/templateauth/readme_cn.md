# templateauth：路由模板选择权限

[English](readme.md) · [上层架构](../readme_cn.md)

## 职责

路由模板选择权限。不执行模板路由；不能只靠前端隐藏保证权限。

## Review 顺序

selection.go: exported validators → callers

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

authz/IAM 与注入查询约束模板选择。

## 验证

同目录测试：selection_test.go

仓库根目录执行：go test ./internal/gateway/templateauth -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
