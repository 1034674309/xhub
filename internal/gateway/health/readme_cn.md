# health：存活、就绪与控制台启动配置

[English](readme.md) · [上层架构](../readme_cn.md)

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

组装模块时注入非 nil Pinger 和公开版本，把返回 Module 挂到 Registrar。存活探针验证进程；就绪探针检查 Ping；版本详情并不执行同样的数据库探测。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| Module(db, version, sso...) | 返回存活、就绪和启动配置模块 |
| PublicOrigin() | 环境变量优先，否则 localhost:4000 |

### 接入 demo

将下面代码保存为仓库内的 `demo/main.go`，在仓库根目录执行 `go run ./demo`；运行后删除本次创建的 demo 目录。

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/gateway/health"
	"net/http"
	"net/http/httptest"
)

type demoPinger struct{}

// Ping 模拟健康依赖；无参数，返回 nil，仅供无网络模块演示。
func (demoPinger) Ping() error { return nil }

type registrar struct{ mux *http.ServeMux }

// Handle 将模块路由适配到 ServeMux；参数为路径与处理器，无返回值，修改本机路由表。
func (r registrar) Handle(pattern string, handler http.HandlerFunc) {
	r.mux.HandleFunc(pattern, handler)
}

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	mux := http.NewServeMux()
	health.Module(demoPinger{}, "demo-version").Mount(registrar{mux})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/health/readiness", nil))
	fmt.Println(w.Code)
}
```

输出 200；替换成返回错误的 Pinger 会得到 503。这是模块接入 demo，真实就绪应注入真实数据库。

### 错误、状态与副作用

Module 在构造时保存 db.Ping 方法，实际 Ping 在就绪请求到来时执行；nil 接口依赖不可用。util.Registrar.Handle 接收 HandlerFunc，ServeMux 需像示例那样适配；详情不能当就绪探测。

## 职责

存活、就绪与控制台启动配置。不依赖 Server、不改模型、不记账；存活不查数据库，就绪失败返回 503。

## Review 顺序

health.go: Pinger → Module → healthLive / healthReady / uiConfig

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

只注入 Ping 和公开版本，PublicOrigin 读取公开地址。

## 验证

同目录测试：health_test.go

仓库根目录执行：go test ./internal/gateway/health -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
