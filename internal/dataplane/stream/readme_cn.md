# stream：原始流转发与只读用量观察

[English](readme.md) · [上层架构](../readme_cn.md)

## 职责

原始流转发与只读用量观察。不重编码原生事件；日志捕获上限不能截断客户端字节，读错误必须传给调用方。

## Review 顺序

stream.go: PipeStream → streamUsageParser → ResponseID

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

execution 调用，usage 规范化统计；改变协议的 SSE 在 dialogue。

## 验证

同目录测试：responses_continuation_test.go, usage_stream_test.go

仓库根目录执行：go test ./internal/dataplane/stream -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
