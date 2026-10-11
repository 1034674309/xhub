# billing：部署费率与一次调用费用计算

[English](readme.md) · [上层架构](../readme_cn.md)

## 职责

部署费率与一次调用费用计算。不读正文、不扣预算、不落库；手工缺价不能默默回落目录。

## Review 顺序

cost.go: DeploymentFinder → CallCost → deploymentCost

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

只依赖部署查找；catalog 统一计算时段、缓存、图片和视频价格；spend.go 落库。

## 验证

同目录测试：cost_test.go

仓库根目录执行：go test ./internal/gateway/billing -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
