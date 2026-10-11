# contract：执行器能力接口和调用事实

[English](readme.md) · [上层架构](../readme_cn.md)

## 职责

执行器能力接口和调用事实。不转发 HTTP、不选路由、不写数据库。

## Review 顺序

host.go: RequestHost → Adapted / Bypass → SpendLog, RoutePlan, CallNote

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

execution 消费，gateway 实现；auth、config、router、prefs、provider 提供值类型。

## 验证

同目录测试：本包仅接口声明，通过组合编译和后台回归验证。

仓库根目录执行：go test ./internal/dataplane/contract -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
