# upstreamurl

[English](readme.md)

为入口契约展示构造显式上游 URL。测试覆盖前缀、尾斜杠、非法 URL 和路径越界。

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

构造兼容上游地址时，把可信部署 api_base、固定 endpoint 和 provider 标识交给 PassthroughURL；随后仍由 transport 负责鉴权与发送。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| PassthroughURL(base, endpoint, provider) | 保留 base 根路径并拼接固定入口 |

### 接入 demo

将下面代码保存为仓库内的 `demo/main.go`，在仓库根目录执行 `go run ./demo`；运行后删除本次创建的 demo 目录。

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/gateway/upstreamurl"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	fmt.Println(upstreamurl.PassthroughURL("https://api.openai.com", "/chat/completions", "openai"))
}
```

输出 https://api.openai.com/v1/chat/completions；不发网络请求。

### 错误、状态与副作用

仅官方 OpenAI 主机补齐 /v1，不能统一覆盖中转站根路径。无效解析可能返回原 base，调用方仍需校验部署 URL。

## 源码与验证

go test ./internal/gateway/upstreamurl
