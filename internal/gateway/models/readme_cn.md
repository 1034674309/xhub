# models：部署、凭据、端点能力与路由配置管理

[English](readme.md) · [上层架构](../readme_cn.md)

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

宿主注入部署存储、鉴权、锁和协议探测；挂载后先验证声明再保存部署，发现接口只展示调用方可访问的可执行端点。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| Module(host) | 构造 util.Module，经 Mount/Server.Use 安装 |
| GET /v2/model/info | 本示例使用的 HTTP 入口 |

### 接入 demo

启动接线可以使用下面的包间接入函数。宿主必须实现表中的接口；reg 可传网关 Server。若使用标准库 ServeMux，按 [health demo](../health/readme_cn.md) 适配 Handle。该函数可编译，实际执行依赖完整宿主；下面的 HTTP demo 用于检查挂载后的接口。

```go
package demo

import (
	"github.com/sunqirui1987/xhub/internal/gateway/models"
	"github.com/sunqirui1987/xhub/internal/util"
)

// MountDemo 演示启动时挂载模块；h 提供真实宿主能力，reg 接收路由，返回无。
// 调用方须先初始化依赖；挂载会修改路由表，不执行 HTTP 请求。
func MountDemo(h models.Host, reg util.Registrar) { models.Module(h).Mount(reg) }
```

先启动已配置的本地网关。设置 XHUB_DEMO_TOKEN 为具有本接口权限的会话令牌。完整宿主接线见 [wire.go](../wire.go) 和 [routes.go](../routes.go)，不可用 nil 依赖代替。

在仓库根目录执行以下命令。

```sh
curl -sS -i "http://localhost:4000/v2/model/info" \
  -H "Authorization: Bearer $XHUB_DEMO_TOKEN"
```

部署管理视图；已登录且有读取权限时返回部署及端点信息。

### 错误、状态与副作用

transport 与 endpoint_types 必须兼容。删除部署要同步清理权重与回退悬空引用；实际探测可能访问供应商并产生费用。

## 职责

部署、凭据、端点能力与路由配置管理。不执行每次用户推理；先看声明校验，再看写入、发现和删除清理。

## Review 顺序

host.go / mount.go → admin.go / validate.go → available.go / discovery.go → groups.go / fallback.go / routing_groups.go → price.go / schedule.go

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

config/provider 声明模型与能力，catalog 提供价格，Host 注入存储、鉴权和探测。

## 验证

同目录测试：admin_test.go, available_test.go, builtin_test.go, discovery_test.go, fallback_test.go, groups_test.go, list_test.go, price_selection_test.go, price_test.go, routing_groups_test.go, schedule_test.go, validate_credential_test.go

仓库根目录执行：go test ./internal/gateway/models -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
