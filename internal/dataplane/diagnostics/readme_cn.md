# diagnostics：上游原始事实与安全诊断日志

[English](readme.md) · [上层架构](../readme_cn.md)

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

在转发读取 Body 之前安装 ObserveUpstream，读取至 EOF 后使用诊断事实；出站请求可先用 TraceFirstByte 记录网络首字节。调用方始终负责关闭 Body。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| TraceFirstByte(req) | 返回带追踪请求与时间指针 |
| ObserveUpstream(resp) | 包装 Body，返回随读取更新的事实 |

### 接入 demo

将下面代码保存为仓库内的 `demo/main.go`，在仓库根目录执行 `go run ./demo`；运行后删除本次创建的 demo 目录。

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/dataplane/diagnostics"
	"io"
	"net/http"
	"strings"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"usage":{"prompt_tokens":0}}`))}
	facts := diagnostics.ObserveUpstream(resp)
	defer resp.Body.Close()
	if _, err := io.ReadAll(resp.Body); err != nil {
		panic(err)
	}
	fmt.Println(facts.StatusCode, facts.UsageReported)
}
```

输出 200 true；明确零也是上报事实。该夹具无网络请求。

### 错误、状态与副作用

未读完或解析超限时事实不完整，不能补成成功用量。日志必须脱敏，原始诊断不承担访问授权或计价。

## 职责

上游原始事实与安全诊断日志。不计算价格，不保存生成内容；路由日志不得泄漏密钥。

## Review 顺序

upstream_diagnostics.go → log.go → host.go: TraceHop

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

execution、transport 采集事实，gateway/requestlog 脱敏并序列化。

## 验证

同目录测试：upstream_diagnostics_test.go

仓库根目录执行：go test ./internal/dataplane/diagnostics -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
