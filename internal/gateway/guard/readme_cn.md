# guard：护栏配置、规则与外部执行器

[English](readme.md) · [上层架构](../readme_cn.md)

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

挂载 Module 提供管理调试；真实推理在上游发送前调用 Evaluate/PreCall，读取 blocked 与 findings，并继续使用脱敏后的 body。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| Module(host) | 构造 util.Module，经 Mount/Server.Use 安装 |
| POST /apply_guardrail | 本示例使用的 HTTP 入口 |

### 接入 demo

启动接线可以使用下面的包间接入函数。宿主必须实现表中的接口；reg 可传网关 Server。若使用标准库 ServeMux，按 [health demo](../health/readme_cn.md) 适配 Handle。该函数可编译，实际执行依赖完整宿主；下面的 HTTP demo 用于检查挂载后的接口。

```go
package demo

import (
	"github.com/sunqirui1987/xhub/internal/gateway/guard"
	"github.com/sunqirui1987/xhub/internal/util"
)

// MountDemo 演示启动时挂载模块；h 提供真实宿主能力，reg 接收路由，返回无。
// 调用方须先初始化依赖；挂载会修改路由表，不执行 HTTP 请求。
func MountDemo(h guard.Host, reg util.Registrar) { guard.Module(h).Mount(reg) }
```

先启动已配置的本地网关。设置 XHUB_DEMO_TOKEN 为具有本接口权限的会话令牌。完整宿主接线见 [wire.go](../wire.go) 和 [routes.go](../routes.go)，不可用 nil 依赖代替。 先通过管理界面保存名为 demo-secrets 的本地 redact 规则，blocked_words 设置为 ["secret"]；示例仅调试该规则，不启用默认执行。

在仓库根目录执行以下命令。

```sh
curl -sS -i -X POST "http://localhost:4000/apply_guardrail" \
  -H "Authorization: Bearer $XHUB_DEMO_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"guardrail_name":"demo-secrets","text":"Hello secret"}'
```

规则正确配置时返回 action=redact、blocked=false、response_text="Hello [REDACTED]"；未保存指定规则返回 404。

### 错误、状态与副作用

Evaluate 可能原地脱敏正文。外部执行器故障需按实际失败策略处理；调试规则不等于启用规则，不能忽略 blocked 或吞掉错误。

## 职责

护栏配置、规则与外部执行器。不路由模型、不计价；外部错误、秘密脱敏、拦截需一起 review。

## Review 顺序

host.go / mount.go → manage.go → guard.go / runner.go → custom_http.go / external.go

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

Host、存储、内置规则和外部执行器组合，gateway 钩子执行结果。

## 验证

同目录测试：custom_test.go, engine_test.go, external_test.go, guard_regression_test.go, guard_test.go, primitives_test.go

仓库根目录执行：go test ./internal/gateway/guard -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
