# compliance：请求日志合规检查接口

[English](readme.md) · [上层架构](../readme_cn.md)

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

启动时提供 RequireManage 的 Host 并挂载在目录兜底前。客户端提交请求日志事实，处理器返回 checks 与 compliant；不会执行实际护栏或保存记录。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| Module(host) | 构造 util.Module，经 Mount/Server.Use 安装 |
| POST /compliance/gdpr | 本示例使用的 HTTP 入口 |

### 接入 demo

启动接线可以使用下面的包间接入函数。宿主必须实现表中的接口；reg 可传网关 Server。若使用标准库 ServeMux，按 [health demo](../health/readme_cn.md) 适配 Handle。该函数可编译，实际执行依赖完整宿主；下面的 HTTP demo 用于检查挂载后的接口。

```go
package demo

import (
	"github.com/sunqirui1987/xhub/internal/gateway/compliance"
	"github.com/sunqirui1987/xhub/internal/util"
)

// MountDemo 演示启动时挂载模块；h 提供真实宿主能力，reg 接收路由，返回无。
// 调用方须先初始化依赖；挂载会修改路由表，不执行 HTTP 请求。
func MountDemo(h compliance.Host, reg util.Registrar) { compliance.Module(h).Mount(reg) }
```

先启动已配置的本地网关。设置 XHUB_DEMO_TOKEN 为具有本接口权限的会话令牌。完整宿主接线见 [wire.go](../wire.go) 和 [routes.go](../routes.go)，不可用 nil 依赖代替。

在仓库根目录执行以下命令。

```sh
curl -sS -i -X POST "http://localhost:4000/compliance/gdpr" \
  -H "Authorization: Bearer $XHUB_DEMO_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"request_id":"demo-request"}'
```

带 compliant、regulation、checks 的检查结果；缺少事实的演示记录会有未通过项，不保证 compliant=true。

### 错误、状态与副作用

request_id 必填；缺失返回 400。管理鉴权失败提前结束，结果只是基于提交事实的规则评估，不等价于法律合规证明。

## 职责

请求日志合规检查接口。只计算提交记录的规则结果，不执行护栏、不持久化，也不替代法律评估。

## Review 顺序

compliance.go: Host / Module → writeCompliance → euAIActChecks / gdprChecks

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

最小 Host 仅管理鉴权，util 挂路由与输出。

## 验证

同目录测试：compliance_test.go

仓库根目录执行：go test ./internal/gateway/compliance -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
