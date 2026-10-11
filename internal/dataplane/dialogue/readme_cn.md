# dialogue：统一对话转换、续接与适配 SSE

[English](readme.md) · [上层架构](../readme_cn.md)

## 职责

统一对话转换、续接与适配 SSE。只转换支持的文本和函数工具，不可表达的能力明确失败；完整原生能力走 bypass。

## Review 顺序

dialogue.go: BuildDialogue, ResponsesRequestBody, ProbeDialogue → dialogue_stream.go: PipeDialogue

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

llm 管理协议值和官方 SDK 类型，provider 声明传输；原生透传在 transport。

## 验证

同目录测试：dialogue_probe_test.go, dialogue_stream_test.go, registered_operation_test.go, responses_continuation_test.go, sdk_stream_test.go

仓库根目录执行：go test ./internal/dataplane/dialogue -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
