# keys：平台与个人 API 密钥生命周期

[English](readme.md) · [上层架构](../readme_cn.md)

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

先在启动接线实现 Host，挂载密钥模块；管理请求先鉴权和逐项授权，再创建、更新、撤销或轮换密钥。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| Module(host) | 构造 util.Module，经 Mount/Server.Use 安装 |
| GET /key/list | 本示例使用的 HTTP 入口 |

### 接入 demo

启动接线可以使用下面的包间接入函数。宿主必须实现表中的接口；reg 可传网关 Server。若使用标准库 ServeMux，按 [health demo](../health/readme_cn.md) 适配 Handle。该函数可编译，实际执行依赖完整宿主；下面的 HTTP demo 用于检查挂载后的接口。

```go
package demo

import (
	"github.com/sunqirui1987/xhub/internal/gateway/keys"
	"github.com/sunqirui1987/xhub/internal/util"
)

// MountDemo 演示启动时挂载模块；h 提供真实宿主能力，reg 接收路由，返回无。
// 调用方须先初始化依赖；挂载会修改路由表，不执行 HTTP 请求。
func MountDemo(h keys.Host, reg util.Registrar) { keys.Module(h).Mount(reg) }
```

先启动已配置的本地网关。设置 XHUB_DEMO_TOKEN 为具有本接口权限的会话令牌。完整宿主接线见 [wire.go](../wire.go) 和 [routes.go](../routes.go)，不可用 nil 依赖代替。

在仓库根目录执行以下命令。

```sh
curl -sS -i "http://localhost:4000/key/list" \
  -H "Authorization: Bearer $XHUB_DEMO_TOKEN"
```

可见密钥列表；返回只包含当前授权范围内记录，不能以此获取明文秘密。

### 错误、状态与副作用

生成与轮换可能返回一次性秘密；保存哈希，不记录明文。撤销及过期必须在下一次鉴权中生效，列表需先应用 Scope。

## 职责

平台与个人 API 密钥生命周期。不发推理；创建、撤销、过期、权限和秘密返回范围需一起审核。

## Review 顺序

host.go → mount.go → generate.go → admin.go / personal.go

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

Host 注入身份及存储，auth/IAM 管理哈希与范围。

## 验证

同目录测试：key_input_test.go, personal_test.go

仓库根目录执行：go test ./internal/gateway/keys -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
