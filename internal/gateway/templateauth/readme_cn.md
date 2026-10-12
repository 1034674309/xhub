# templateauth：路由模板选择权限

[English](readme.md) · [上层架构](../readme_cn.md)

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

密钥、团队、组织保存或绑定模板前，先 Selection 验证模板可见性。成功后才修改作用域；空 ID 表示继承而不是创建空模板。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| Host.Identity / Host.Authorize | 读取真实模板归属并执行授权 |
| Selection(host, request, principal, id) | 返回错误；空 ID 允许继承 |

### 接入 demo

这是供宿主调用的 Go 接入函数。将代码放在仓库内独立包中可编译；实际执行前须由调用方提供下述真实依赖。

```go
package demo

import (
	"github.com/sunqirui1987/xhub/internal/auth"
	"github.com/sunqirui1987/xhub/internal/gateway/templateauth"
	"net/http"
)

// ValidateSelection 演示保存前的检查；传入真实宿主、请求、已鉴权身份及模板 ID，返回错误，不执行保存。
func ValidateSelection(h templateauth.Host, r *http.Request, p *auth.Principal, templateID string) error {
	return templateauth.Selection(h, r, p, templateID)
}
```

函数可编译；非空 ID 需要真实 IAM。nil 表示本次选择可继续保存，保存由调用方完成。

### 错误、状态与副作用

模板不存在返回 NotFound，读取失败返回 InternalError，不可见返回授权错误。不能把客户端声明的组织或团队当归属。

## 职责

路由模板选择权限。不执行模板路由；不能只靠前端隐藏保证权限。

## Review 顺序

selection.go: exported validators → callers

先读消费方接口或输入契约，再沿入口阅读辅助函数及旁边的单元测试；涉及请求生命周期时检查上层调用方。

## 依赖

authz/IAM 与注入查询约束模板选择。

## 验证

同目录测试：selection_test.go

仓库根目录执行：go test ./internal/gateway/templateauth -count=1

真实 HTTP/持久化契约放 cmd/regression；浏览器 → 网关 → 本地上游流程放 frontend/e2e。测试清理私有 schema 与临时夹具；离线测试不假设有付费供应商凭据。
