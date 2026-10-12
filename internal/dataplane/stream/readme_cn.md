# stream：原始流转发与只读用量观察

[English](readme.md) · [上层架构](../readme_cn.md)

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

原生 SSE 由 PipeStream 一次转发；它返回完成事实、用量、首字耗时和日志捕获，执行器据此记录结果。跨协议事件转换应调用 dataplane/dialogue，而不是重写这里的原始流。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| PipeStream(w, resp, start) | 写客户端原字节并返回观测结果及错误 |
| ResponseID(raw) | 读取响应 JSON 或完整 SSE 中的 ID |

### 接入 demo

将下面代码保存为仓库内的 `demo/main.go`，在仓库根目录执行 `go run ./demo`；运行后删除本次创建的 demo 目录。

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/dataplane/stream"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	fmt.Println(stream.ResponseID([]byte(`{"id":"demo-response","object":"response"}`)))
}
```

输出 demo-response；这是离线 ID 提取示例。PipeStream 的实际字节保真与读错误见同目录测试。

### 错误、状态与副作用

日志截断不能截断客户端输出；读错误必须上交执行器。流已输出后不能拼接另一个供应商响应，也不能把失败流发布成可续接历史。

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
