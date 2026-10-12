# identity：用户、团队成员与管理权限

[English](readme.md) · [上层架构](../readme_cn.md)

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

宿主实现 Gate 的身份、授权及存储能力，启动时挂载；处理器从数据库恢复真实归属，成员、角色、模板绑定各自授权后修改。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| Module(gate) | 构造 util.Module，经 Mount/Server.Use 安装 |
| GET /user/info | 本示例使用的 HTTP 入口 |

### 接入 demo

启动接线可以使用下面的包间接入函数。宿主必须实现表中的接口；reg 可传网关 Server。若使用标准库 ServeMux，按 [health demo](../health/readme_cn.md) 适配 Handle。该函数可编译，实际执行依赖完整宿主；下面的 HTTP demo 用于检查挂载后的接口。

```go
package demo

import (
	"github.com/sunqirui1987/xhub/internal/gateway/identity"
	"github.com/sunqirui1987/xhub/internal/util"
)

// MountDemo 演示启动时挂载模块；h 提供真实宿主能力，reg 接收路由，返回无。
// 调用方须先初始化依赖；挂载会修改路由表，不执行 HTTP 请求。
func MountDemo(h identity.Gate, reg util.Registrar) { identity.Module(h).Mount(reg) }
```

先启动已配置的本地网关。设置 XHUB_DEMO_TOKEN 为具有本接口权限的会话令牌。完整宿主接线见 [wire.go](../wire.go) 和 [routes.go](../routes.go)，不可用 nil 依赖代替。

在仓库根目录执行以下命令。

```sh
curl -sS -i "http://localhost:4000/user/info" \
  -H "Authorization: Bearer $XHUB_DEMO_TOKEN"
```

当前身份可见的用户信息；需要有效管理/用户会话及相应权限。

### 错误、状态与副作用

成员或角色改变会影响后续实时授权。绑定路由模板前执行 templateauth.Selection；写入与审计必须保持一致。

## 职责

用户、团队成员与管理权限。不转发推理；先看鉴权，再看成员与模板，防止越权写入。

## Review 顺序

gate.go → mount.go → handlers.go / members.go / route_template.go

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

auth、IAM 提供身份和成员关系；模块边界注入存储与管理。

## 验证

同目录测试：pagination_test.go, route_template_error_test.go

仓库根目录执行：go test ./internal/gateway/identity -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
