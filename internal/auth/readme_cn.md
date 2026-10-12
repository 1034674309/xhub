# 请求身份认证

[English](readme.md)

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

HTTP 准入先用 Resolve 解析真实凭据；界面会话先验证签名，再用 SessionPrincipal 恢复用户。把得到的 Principal 交给 authz，不能把成功鉴权当作动作授权。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| APIKeyFrom(r) | 提取请求凭据，不访问存储 |
| Resolve(ctx, cfg, db, r) | 返回 Principal 或鉴权/依赖错误 |
| SessionPrincipal(user, version, session) | 校验用户会话状态并构造身份 |

### 接入 demo

将下面代码保存为仓库内的 `demo/main.go`，在仓库根目录执行 `go run ./demo`；运行后删除本次创建的 demo 目录。

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/auth"
	"net/http/httptest"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	r := httptest.NewRequest("GET", "/v1/models", nil)
	r.Header.Set("Authorization", "Bearer demo-key")
	fmt.Println(auth.APIKeyFrom(r) == "demo-key")
}
```

输出 true，只演示凭据提取；完整 Resolve 还需要已加载配置和真实 IAM。

### 错误、状态与副作用

依赖错误应保留为服务端错误；不能把数据库故障当作密钥不存在。不要记录提取出的明文凭据。

## 说明

`internal/auth` 把 HTTP 请求携带的凭据或已验证的界面会话转换为 `Principal`。它只回答“调用者是谁”，具体动作能否执行由 `internal/authz` 决定。主调用方既包含鉴权所需的身份字段，也保留路由、预算、限流、花费、模型访问和请求日志后续需要的密钥信息，可表示应急主密钥、虚拟 API 密钥或已登录用户会话。

本包不会保存明文密钥。`Resolve` 先提取请求中的明文，只对显式配置的主密钥做精确比较；其他值先计算哈希，再查询 IAM 密钥行。界面会话不经过 `Resolve`：网关先验证签名令牌，再把当前用户行、令牌中的版本和会话标识交给 `SessionPrincipal`。

## 对外接口

- `APIKeyFrom(*http.Request) string` 按固定优先级提取一个凭据：`x-litellm-api-key`、Bearer 或 Basic 形式的 `Authorization`、`api-key`、`x-api-key`。Basic 内容经标准 base64 解码后，存在冒号时取第一个冒号后的密码，否则取完整解码结果。只有 `/v1beta/models/` 与 `/vertex/v1/models/` 路径还接受 `x-goog-api-key`，最后才读取 `key` 查询参数。空值或格式错误的候选项会继续尝试低优先级来源；没有可用凭据时返回空串，不访问数据库。
- `Resolve(context.Context, *config.Config, *iam.DB, *http.Request) (*Principal, error)` 认证主密钥和虚拟密钥。成功时返回主密钥调用方，或包含 IAM 行、SHA-256 查询哈希、归属 ID 与限额信息的密钥调用方。未提供凭据、没有存储实例、哈希不存在时返回包装 `authz.ErrUnauthenticated` 的 `invalid_api_key`；密钥已吊销、被封禁或处于其他非活动状态、已过期时分别返回包装 `authz.ErrForbidden` 的 `key_revoked`、`key_blocked`、`key_expired`。存储读取失败保留为 `authz.InternalError`，网关据此返回服务端错误，不会伪装成错误密钥。上下文可取消查询；日志只记录判定结果和标识，不记录明文。
- `SessionPrincipal(*iam.User, version, session) (*Principal, error)` 只接受当前仍为活动状态、且数据库 `session_version` 与签名令牌版本完全一致的用户。成功结果包含用户 ID、平台角色、会话 ID 和版本。用户不存在、被停用、被删除或版本落后时，与 `ErrSessionInvalid()` 一样返回未认证错误。因此，修改密码或其他递增会话版本的操作会在下一次请求使全部旧会话失效。
- `Principal` 是交给后续链路的身份对象。`Actor()` 只投影授权层使用的字段，nil 接收者返回零值 Actor；`IsMaster()` 判断应急主密钥；`PlatformAdmin()` 只对管理员会话成立，管理员拥有的密钥不会继承管理权限；`CanInfer()` 对会话和密钥类型成立，但密钥在执行受保护操作前仍由 authz 重查有效性。结构中还预留团队、项目、所有者、结算团队与组织快照、已选路由模板及来源、会话信息，由网关预算与模板解析阶段补齐或使用。
- `Error` 把 authz 错误类别与稳定客户端错误码绑定。`Code(error)` 可取回该码，无法识别的错误默认得到 `invalid_api_key`；调用方仍应先判断 `authz.IsInternal`，避免把依赖故障当成凭据错误。

主要调用者包括网关会话与请求引擎、数据面推理准入、模型访问与身份处理、密钥管理、路由模板选择、限流与花费结算。它们会补充或读取 `Principal`，但不能绕过 authz 的实时授权检查。

## 内部核心实现

凭据提取严格遵循优先级，首个非空且格式可接受的来源立即胜出，防止低优先级兼容头覆盖 LiteLLM 或 `Authorization` 凭据。Bearer 前缀不区分大小写，并清理两端空白。Basic 先清理整个头再截取内容，使用标准 base64 解码，并且只按第一个冒号拆分，因此密码本身可以包含冒号。Basic 解码失败不会直接报错，而是继续检查后续头。Google 查询参数受路径限制，其他接口不会从 URL 读取密钥。

`Resolve` 首先拒绝空提取结果，然后仅在配置主密钥非空时做精确匹配；命中后直接生成 `KindMaster`，不查询 IAM。普通密钥必须有存储实例，明文经 `iam.HashKey` 计算后按哈希查行，再判断生命周期。状态判断早于过期判断：同时“已吊销且过期”的密钥报告 `key_revoked`；其他非活动状态统一报告 `key_blocked`。过期边界采用“当前时间不再早于 `ExpiresAt`”即拒绝。成功转换时复制可空用户与项目 ID，并保留原密钥行和哈希，供后续预算、限流和记账使用。

认证结果不会冻结权限。此处接受的密钥在受保护请求中仍由 `authz.Guard.CheckKey` 重新加载，因此吊销、所有者停用、成员关系移除、团队或组织封禁、项目停用都会在下一次请求生效。会话构造同样使用实时用户行核对签名版本。数据库读取失败按内部错误关闭访问；不存在的哈希与普通无效密钥故意保持相同外部表现。

### 测试依赖

`auth_test.go` 验证 Basic 头两端空白会在截取和解码前清理。`google_key_test.go` 验证 Google 原生头、查询参数及其路径限制。生命周期优先级、依赖失败、Principal 投影和会话失效目前主要通过调用方与 authz 测试覆盖，本目录没有对应的直接单测。

## 源码职责与入口

- [auth.go](auth.go) 是唯一生产源码，定义稳定错误码与 `Error` 包装契约、`Principal` 及其授权视图、兼容凭据提取、主密钥与虚拟密钥解析、生命周期分类、密钥行转换和实时用户会话构造。理解线格式优先级从 `APIKeyFrom` 开始，理解密钥认证从 `Resolve` 开始，理解界面会话失效从 `SessionPrincipal` 开始。
- [auth_test.go](auth_test.go) 验证 Basic 凭据空白处理。
- [google_key_test.go](google_key_test.go) 验证仅限 Gemini/Vertex 路径的 Google 凭据形式。
