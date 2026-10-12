# registry

[English](readme.md)

内置注册与路径匹配.

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

进程空白导入 provider/all 后，入口用 Match(method,path) 获取 Hit；再由 deployment 验证所选部署。RegisterTransport/Model/Supplier 供启动登记，不能让请求上传任意协议路径。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| RegisterTransport / RegisterModel / RegisterSupplier | 登记内置协议与目录贡献 |
| Match(method, path) | 返回 Hit,bool；未知路径不命中 |
| Transports / PublicBody | 返回协议目录视图 |

### 接入 demo

将下面代码保存为仓库内的 `demo/main.go`，在仓库根目录执行 `go run ./demo`；运行后删除本次创建的 demo 目录。

```go
package main

import (
	"fmt"
	_ "github.com/sunqirui1987/xhub/internal/provider/all"
	"github.com/sunqirui1987/xhub/internal/provider/registry"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	hit, ok := registry.Match("POST", "/bypass/openai/v1/responses")
	if !ok {
		panic("registered action not found")
	}
	fmt.Println(hit.Transport.ID, hit.Action.Name)
}
```

输出 bypass_openai_responses create。示例只验证协议登记，不需要账号、不发送任务。

### 错误、状态与副作用

全局注册应在启动完成前结束；Transports 返回复制值。匹配路径不是授权，也不代表配置了可用部署。 导入不会连接服务、创建凭据或证明供应商在线；异步任务费用必须由数据面去重。

## 源码与验证

代码入口：[catalog_binding.go](catalog_binding.go), [match.go](match.go), [models.go](models.go), [registry.go](registry.go), [suppliers.go](suppliers.go)

单元测试与源代码放在一起：[boundaries_test.go](boundaries_test.go), [catalog_binding_test.go](catalog_binding_test.go), [endpoint_redesign_test.go](endpoint_redesign_test.go), [isolation_test.go](isolation_test.go), [match_test.go](match_test.go), [registry_test.go](registry_test.go)

go test ./internal/provider/registry/...

注册和读取均复制可变切片、映射和计量回调容器。Match 仅接受后台固定动作，不读取部署上传路径。

真实后台回归在 cmd/regression，浏览器流程在 frontend/e2e。离线测试使用本地 HTTP 上游，不证明生产凭据或外部厂商服务可用性。

## 覆盖率检查

运行 scripts/core-codec-coverage.sh（使用 bash），注册表和对话编解码各自必须达到 95% 语句覆盖率。测试失败、缺少报告或低于下限都会返回失败。覆盖率来自各包自身的单元测试，不混入后台回归或浏览器测试。
