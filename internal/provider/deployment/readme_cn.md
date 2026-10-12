# deployment

[English](readme.md)

显式绑定、校验与候选筛选.

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

启动和管理写入时 ValidateDeployment，运行时 Execution/AllowsEndpoint/ResolveHit 再校验当前配置。输入必须明确 model_info.transport 与 endpoint_types，不能只填写供应商和地址。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| ValidateDeployment(model) | 错误时拒绝保存或执行 |
| Execution(model) | 返回显式绑定传输和 bool |
| Candidates / DeploymentEndpoints | 过滤候选并生成入口视图 |

### 接入 demo

将下面代码保存为仓库内的 `demo/main.go`，在仓库根目录执行 `go run ./demo`；运行后删除本次创建的 demo 目录。

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/config"
	_ "github.com/sunqirui1987/xhub/internal/provider/all"
	"github.com/sunqirui1987/xhub/internal/provider/deployment"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	model := config.ModelEntry{ModelName: "demo-model", LiteLLMParams: map[string]any{"model": "demo-upstream"}, ModelInfo: map[string]any{"transport": "bypass_openai_responses", "endpoint_types": []string{"bypass:openai-responses"}}}
	if err := deployment.ValidateDeployment(model); err != nil {
		panic(err)
	}
	transport, ok := deployment.Execution(model)
	fmt.Println(ok, transport.ID)
}
```

输出 true bypass_openai_responses；这是协议绑定校验，不测试地址、凭据或网络。

### 错误、状态与副作用

缺失声明不得从 api_base/custom_llm_provider 猜执行方式。能力符合后仍须检查停用、权限、预算与权重。

## 源码与验证

代码入口：[bindings.go](bindings.go), [candidates.go](candidates.go), [catalog_binding.go](catalog_binding.go), [execution.go](execution.go), [registry.go](registry.go), [validate.go](validate.go)

单元测试与源代码放在一起：[bindings_test.go](bindings_test.go), [candidates_test.go](candidates_test.go), [catalog_binding_test.go](catalog_binding_test.go), [endpoint_redesign_test.go](endpoint_redesign_test.go), [registry_test.go](registry_test.go), [validate_test.go](validate_test.go)

go test ./internal/provider/deployment/...

真实后台回归在 cmd/regression，浏览器流程在 frontend/e2e。离线测试使用本地 HTTP 上游，不证明生产凭据或外部厂商服务可用性。
