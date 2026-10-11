# publichub：公开模型目录、筛选排序与链接

[English](readme.md) · [上层架构](../readme_cn.md)

## 职责

公开模型目录、筛选排序与链接。不暴露私有凭据；分页、大小写筛选与存储失败在此处理。

## Review 顺序

public_hub.go: Module → publicModelHub / publicModelHubFacet → filterPublicModels / sortPublicModels

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

catalog 提供公开数据，最小 Host 控制写权限，Store 保存链接 KV。

## 验证

同目录测试：public_hub_test.go

仓库根目录执行：go test ./internal/gateway/publichub -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
