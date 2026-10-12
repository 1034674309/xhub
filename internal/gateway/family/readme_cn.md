# family：目录资源接口与租户隔离

[English](readme.md) · [上层架构](../readme_cn.md)

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

通过 Host 提供身份、作用域存储和数据面派发。Module 只挂载 POST /v1/responses 和 POST /responses；目录兜底由 gateway/catalog.go 另行调用 ServeDataPlane、ServeMixed 和 ServeManagement。先分清推理、混合和管理入口。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| Module(host) | 构造 util.Module，经 Mount/Server.Use 安装 |
| POST /v1/responses | 本示例使用的 HTTP 入口 |

### 接入 demo

启动接线可以使用下面的包间接入函数。宿主必须实现表中的接口；reg 可传网关 Server。若使用标准库 ServeMux，按 [health demo](../health/readme_cn.md) 适配 Handle。该函数可编译，实际执行依赖完整宿主；下面的 HTTP demo 用于检查挂载后的接口。

```go
package demo

import (
	"github.com/sunqirui1987/xhub/internal/gateway/family"
	"github.com/sunqirui1987/xhub/internal/util"
)

// MountDemo 演示启动时挂载模块；h 提供真实宿主能力，reg 接收路由，返回无。
// 调用方须先初始化依赖；挂载会修改路由表，不执行 HTTP 请求。
func MountDemo(h family.Host, reg util.Registrar) { family.Module(h).Mount(reg) }
```

先启动已配置的本地网关。设置 XHUB_DEMO_TOKEN 为具有本接口模型调用权限的会话令牌或虚拟密钥。完整宿主接线见 [wire.go](../wire.go) 和 [routes.go](../routes.go)，不可用 nil 依赖代替。

在仓库根目录执行以下命令。

```sh
curl -sS -i "http://localhost:4000/v1/responses" \
  -H "Authorization: Bearer $XHUB_DEMO_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"model":"demo-model","input":"Hello","stream":false}'
```

配置 demo-model 指向支持 responses 的本地模拟上游，并提供有调用权限的凭据；成功返回 response 对象，失败检查 error。本例会执行一次推理。

### 错误、状态与副作用

目录列出路径不等于通用处理器实现了完整供应商业务。资源保存使用可信 owner，不能以客户端字段绕过隔离。

## 职责

目录资源接口与租户隔离。专门业务接口应由专门模块处理；先看资源隔离再看通用输出。

## Review 顺序

host.go → mount.go → handlers.go

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

Host 注入身份、作用域存储和目录元数据；gateway/catalog.go 注册。

## 验证

同目录测试：pagination_test.go, resource_isolation_test.go

仓库根目录执行：go test ./internal/gateway/family -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
