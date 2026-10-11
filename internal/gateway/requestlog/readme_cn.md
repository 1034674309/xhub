# requestlog：内容编码、流式日志重建与秘密脱敏

[English](readme.md) · [上层架构](../readme_cn.md)

## 职责

内容编码、流式日志重建与秘密脱敏。不计价、不发请求；日志重建不改变客户端响应，原始统计与计费用量可对照。

## Review 顺序

encode.go: Encode → assembleLoggedResponse / redactHeaders → diagnostics.go: DiagnosticJSON

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

输入是值和诊断事实，gateway/spend.go 管理生命周期和落库。

## 验证

同目录测试：encode_test.go, prompt_response_test.go

仓库根目录执行：go test ./internal/gateway/requestlog -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
