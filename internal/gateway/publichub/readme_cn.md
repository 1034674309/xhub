# publichub：公开模型目录、筛选排序与链接

[English](readme.md) · [上层架构](../readme_cn.md)

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

启动时注入管理鉴权与链接存储并挂载；公共读取展示 catalog 目录，修改 useful links 仍要求管理身份。公开目录与用户部署发现是两个不同视图。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| Module(host, store) | 构造 util.Module，经 Mount/Server.Use 安装 |
| GET /public/endpoints | 本示例使用的 HTTP 入口 |

### 接入 demo

启动接线可以使用下面的包间接入函数。宿主必须实现表中的接口；reg 可传网关 Server。若使用标准库 ServeMux，按 [health demo](../health/readme_cn.md) 适配 Handle。该函数可编译，实际执行依赖完整宿主；下面的 HTTP demo 用于检查挂载后的接口。

```go
package demo

import (
	"github.com/sunqirui1987/xhub/internal/gateway/publichub"
	"github.com/sunqirui1987/xhub/internal/util"
)

// MountDemo 演示启动时挂载模块；h 提供真实宿主能力，reg 接收路由，返回无。
// 调用方须先初始化依赖，st 提供真实存储；挂载会修改路由表，不执行 HTTP 请求。
func MountDemo(h publichub.Host, st publichub.Store, reg util.Registrar) {
	publichub.Module(h, st).Mount(reg)
}
```

先启动已配置的本地网关。完整宿主接线见 [wire.go](../wire.go) 和 [routes.go](../routes.go)，不可用 nil 依赖代替。

在仓库根目录执行以下命令。

```sh
curl -sS -i "http://localhost:4000/public/endpoints"
```

返回公开端点目录，无需密钥；这是可声明入口，不代表已配置可调用部署。

### 错误、状态与副作用

公开响应不能包含凭据或租户配置；市场目录与已部署模型不能混为一谈。更新链接会写入存储并受管理鉴权保护。

## 职责

公开模型目录、筛选排序与链接。不暴露私有凭据；分页、大小写筛选与存储失败在此处理。

## Review 顺序

public_hub.go: Module → publicModelHub / publicModelHubFacet → filterPublicModels / sortPublicModels

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

catalog 提供公开数据，最小 Host 控制写权限，Store 保存链接 KV。

## 验证

同目录测试：public_hub_test.go

仓库根目录执行：go test ./internal/gateway/publichub -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
