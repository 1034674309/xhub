# usage：报表、活动、护栏统计与日志读取

[English](readme.md) · [上层架构](../readme_cn.md)

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

宿主同时提供用量 Host 与 identity.Gate，挂载后用 Scope 限定日志查询；单条日志和会话日志仍需单独检查读取动作。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| Module(host) | 构造 util.Module，经 Mount/Server.Use 安装 |
| GET /spend/logs/ui | 本示例使用的 HTTP 入口 |

### 接入 demo

启动接线可以使用下面的包间接入函数。宿主必须实现表中的接口；reg 可传网关 Server。若使用标准库 ServeMux，按 [health demo](../health/readme_cn.md) 适配 Handle。该函数可编译，实际执行依赖完整宿主；下面的 HTTP demo 用于检查挂载后的接口。

```go
package demo

import (
	"github.com/sunqirui1987/xhub/internal/gateway/identity"
	"github.com/sunqirui1987/xhub/internal/gateway/usage"
	"github.com/sunqirui1987/xhub/internal/util"
)

// UsageHost 合并模块挂载所需的用量查询与身份授权接口，由生产宿主实现。
type UsageHost interface {
	usage.Host
	identity.Gate
}

// MountDemo 演示启动时挂载模块；h 提供真实宿主能力，reg 接收路由，返回无。
// 调用方须先初始化依赖；挂载会修改路由表，不执行 HTTP 请求。
func MountDemo(h UsageHost, reg util.Registrar) { usage.Module(h).Mount(reg) }
```

先启动已配置的本地网关。设置 XHUB_DEMO_TOKEN 为具有本接口权限的会话令牌。完整宿主接线见 [wire.go](../wire.go) 和 [routes.go](../routes.go)，不可用 nil 依赖代替。

在仓库根目录执行以下命令。

```sh
curl -sS -i "http://localhost:4000/spend/logs/ui" \
  -H "Authorization: Bearer $XHUB_DEMO_TOKEN"
```

已授权的分页调用日志；可能为空，示例不创建消费记录。

### 错误、状态与副作用

费用各作用域不能相加；提示词显示受配置和权限约束。查询与删除要基于可信作用域，不能跨租户读取日志。

## 职责

报表、活动、护栏统计与日志读取。不产生账单、不调供应商；先看租户读权限，再看筛选、分页与金额。

## Review 顺序

host.go / mount.go → reports.go → activity.go / entity_activity.go → guardrails.go

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

Host 提供作用域身份和存储，IAM/store 查询持久化数据。

## 验证

同目录测试：activity_test.go, cost_breakdown_test.go, guardrails_test.go, health_connection_test.go, log_guardrail_test.go, reports_test.go

仓库根目录执行：go test ./internal/gateway/usage -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
