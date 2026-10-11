# transport：原生 HTTP、正文、路径、鉴权与响应透传

[English](readme.md) · [上层架构](../readme_cn.md)

## 职责

原生 HTTP、正文、路径、鉴权与响应透传。保留供应商扩展 JSON、multipart、状态、错误、SSE 字节；不引入 SDK 隐式重试。

## Review 顺序

bypass_body.go → bypass_url.go / official.go → bypass_forward.go: ForwardOfficial, WriteThrough

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

注入 http.Client 与 provider.Hit/Transport，diagnostics 保存事实，不依赖 Server。

## 验证

同目录测试：endpoint_redesign_test.go, fallback_test.go, native_bypass_test.go

仓库根目录执行：go test ./internal/dataplane/transport -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
