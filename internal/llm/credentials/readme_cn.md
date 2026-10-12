# credentials

[English](readme.md)

可信凭据水合.

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

从可信存储取命名凭据，再 Hydrate 到当前部署参数，随后校验 transport 与连接配置。每次需要新快照时调用，不把结果回写到原部署映射。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| Hydrate(params, credentialValues) | 返回新顶层映射，展开环境变量 |

### 接入 demo

将下面代码保存为仓库内的 `demo/main.go`，在仓库根目录执行 `go run ./demo`；运行后删除本次创建的 demo 目录。

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/llm/credentials"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	params := map[string]any{"model": "demo-model", "api_key": "old-demo-key"}
	values := map[string]any{"api_key": "new-demo-key", "custom_llm_provider": "OpenAI"}
	hydrated := credentials.Hydrate(params, values)
	fmt.Println(hydrated["custom_llm_provider"], params["api_key"] == "old-demo-key")
}
```

输出 openai true；新密钥替换结果中的旧值，原输入保留。示例不打印密钥。

### 错误、状态与副作用

仅复制凭据白名单 api_key/api_base/custom_llm_provider；非 key 字段只填空。这是浅复制，嵌套值仍共享；不提供 AWS/Azure 自动认证。

## 源码与验证

命名凭据只填入执行层读取的 `api_key`、`api_base` 和 `custom_llm_provider`。非空凭据密钥替换部署密钥，其余字段只补空值；环境变量引用在水合时展开。已有部署参数保持原值。

本层没有 Azure、AWS、Vertex 自动认证执行器，因此不从命名凭据复制这些认证字段。护栏的 AWS 配置由护栏模块独立处理。

代码入口：[credential.go](credential.go)

单元测试与源代码放在一起：[hydrate_test.go](hydrate_test.go)

go test ./internal/llm/credentials/...

真实后台回归在 cmd/regression，浏览器流程在 frontend/e2e。离线测试使用本地 HTTP 上游，不证明生产凭据或外部厂商服务可用性。
