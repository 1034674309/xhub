# requestlog：内容编码、流式日志重建与秘密脱敏

[English](readme.md) · [上层架构](../readme_cn.md)

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

执行器在拿到完整交换事实后调用 Encode，将三个字符串写入已授权的日志存储；上游诊断用 DiagnosticJSON 单独序列化。日志重建不用于客户端响应。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| Encode(r, requestBytes, responseBytes) | 返回 messages/response/proxy JSON 文本 |
| DiagnosticJSON(facts, usage) | 返回脱敏诊断文本 |

### 接入 demo

将下面代码保存为仓库内的 `demo/main.go`，在仓库根目录执行 `go run ./demo`；运行后删除本次创建的 demo 目录。

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/gateway/requestlog"
	"net/http/httptest"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	r := httptest.NewRequest("POST", "/v1/responses", nil)
	r.Header.Set("Authorization", "Bearer demo-key")
	messages, response, proxy := requestlog.Encode(r, []byte(`{"input":"Hello"}`), []byte(`{"id":"demo-response","output":[]}`))
	fmt.Println(messages != "", response != "", proxy != "")
}
```

输出 true true true；返回的是日志字段，示例不输出其中内容。

### 错误、状态与副作用

脱敏不代替日志读取权限；是否保存提示词由上层配置控制。SSE 日志重建不能用于重编码客户端流。

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
