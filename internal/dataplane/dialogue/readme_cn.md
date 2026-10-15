# dialogue：统一对话转换、续接与适配 SSE

[English](readme.md) · [上层架构](../readme_cn.md)

`PipeDialogue` 对同协议 OpenAI 调用保留原生 SSE 并校验终态，其他组合复用统一流状态机；`ResponseHistory` 保留原生推理/工具项。标准入口仍经过鉴权、预算和护栏；不改用 bypass 执行链。完整边界见[兼容对照](../../../docs/development/zh-CN/agent-protocol-compatibility.md)。

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

在 responses 续接恢复历史之后调用 ResponsesRequestBody，把历史与增量合并，再执行护栏和预算；已水合的部署交给 BuildDialogue/BuildRegisteredOperation，最后由执行器发送。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| ResponsesRequestBody(body, history) | 返回合并后的新请求或错误 |
| BuildDialogue / BuildRegisteredOperation | 构造显式 transport 的上游请求 |
| ProbeDialogue(ctx, client, dep) | 发送真实最小探测请求 |

### 接入 demo

将下面代码保存为仓库内的 `demo/main.go`，在仓库根目录执行 `go run ./demo`；运行后删除本次创建的 demo 目录。

```go
package main

import (
	"fmt"
	dpdialogue "github.com/sunqirui1987/xhub/internal/dataplane/dialogue"
	llmdialogue "github.com/sunqirui1987/xhub/internal/llm/dialogue"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	history := []llmdialogue.Turn{{Role: "user", Text: "Hello"}, {Role: "assistant", Text: "Hi"}}
	body, err := dpdialogue.ResponsesRequestBody(map[string]any{"input": "Continue", "previous_response_id": "demo-response"}, history)
	if err != nil {
		panic(err)
	}
	d, err := llmdialogue.ParseDialogue("openai-responses", body)
	if err != nil {
		panic(err)
	}
	fmt.Println(len(d.Turns), body["previous_response_id"] == nil)
}
```

输出 3 true：两条父历史加一条本轮输入，父 ID 已在网关消费。

### 错误、状态与副作用

当前输入仍必须有效。StripGatewayFields 原地修改映射；ProbeDialogue 会访问外部服务并可能产生费用，不能当纯校验调用。

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
