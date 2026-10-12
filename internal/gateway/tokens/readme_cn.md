# tokens：本地 token 估算、参数发现与转换记录

[English](readme.md) · [上层架构](../readme_cn.md)

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

宿主提供混合入口鉴权、当前模型和命名空间存储，启动时挂载。工具接口解析请求模型，用实际计数模型返回估计；完整转换仍需协议校验。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| Module(host, store) | 构造 util.Module，经 Mount/Server.Use 安装 |
| POST /utils/token_counter | 本示例使用的 HTTP 入口 |

### 接入 demo

启动接线可以使用下面的包间接入函数。宿主必须实现表中的接口；reg 可传网关 Server。若使用标准库 ServeMux，按 [health demo](../health/readme_cn.md) 适配 Handle。该函数可编译，实际执行依赖完整宿主；下面的 HTTP demo 用于检查挂载后的接口。

```go
package demo

import (
	"github.com/sunqirui1987/xhub/internal/gateway/tokens"
	"github.com/sunqirui1987/xhub/internal/util"
)

// MountDemo 演示启动时挂载模块；h 提供真实宿主能力，reg 接收路由，返回无。
// 调用方须先初始化依赖，st 提供真实存储；挂载会修改路由表，不执行 HTTP 请求。
func MountDemo(h tokens.Host, st tokens.Store, reg util.Registrar) { tokens.Module(h, st).Mount(reg) }
```

先启动已配置的本地网关。设置 XHUB_DEMO_TOKEN 为有效工具入口凭据，且 demo-model 为已配置模型。完整宿主接线见 [wire.go](../wire.go) 和 [routes.go](../routes.go)，不可用 nil 依赖代替。

在仓库根目录执行以下命令。

```sh
curl -sS -i -X POST "http://localhost:4000/utils/token_counter" \
  -H "Authorization: Bearer $XHUB_DEMO_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"model":"demo-model","messages":[{"role":"user","content":"Hello"}]}'
```

返回计数结果及使用模型；结果依赖部署和 tokenizer，不能作为供应商实际账单。

### 错误、状态与副作用

需要有效工具入口凭据；未知模型和无法转换字段应返回错误。计数接口不发送真实推理，不授予后续推理权限。

## 职责

本地 token 估算、参数发现与转换记录。不发供应商推理，估算不等于账单实测；存储失败应可观察。

## Review 顺序

tokens.go: Host / Store → Module → tokenCounter / supportedOpenAIParams / transformRequest

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

Host 注入鉴权和模型快照，Store 提供 KV，llm/estimate 计数。

## 验证

同目录测试：tokens_test.go

仓库根目录执行：go test ./internal/gateway/tokens -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
