# transport：原生 HTTP、正文、路径、鉴权与响应透传

[English](readme.md) · [上层架构](../readme_cn.md)

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

先匹配已登记 Hit，并由 deployment 验证部署绑定；解析正文、生成固定动作 URL，再使用注入 HTTP client 发送。ForwardOfficial 返回事实，WriteThrough 输出原始响应。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| ParseBypassBody(raw, contentType) | 解析 JSON/multipart，保留原厂字段 |
| BypassDeploymentURL(base, hit) | 构造注册动作地址并编码路径参数 |
| ForwardOfficial / WriteThrough | 发送并原样输出状态、头与正文 |

### 接入 demo

将下面代码保存为仓库内的 `demo/main.go`，在仓库根目录执行 `go run ./demo`；运行后删除本次创建的 demo 目录。

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/dataplane/transport"
	"github.com/sunqirui1987/xhub/internal/provider/contract"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	hit := contract.Hit{Action: contract.Action{UpstreamPath: "/v1/videos/{id}"}, Names: map[string]string{"id": "demo-task"}}
	fmt.Println(transport.BypassDeploymentURL("https://example.com/v1", hit))
}
```

输出 https://example.com/v1/videos/demo-task；纯 URL 示例不访问 example.com。

### 错误、状态与副作用

只允许登记的方法与路径；不能把客户端提供的 URL 当任意代理目标。转发清除客户端凭据，使用部署凭据；SDK 隐式重试不能叠加在此层。

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
