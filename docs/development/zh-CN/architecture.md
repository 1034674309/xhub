# 请求链与模块边界

~~~text
Console :3000 ─┐
Application ──┴─> Gateway :4000
                  -> Auth / Authz / Limits
                  -> Router -> Dataplane -> Provider
                  -> Usage -> PostgreSQL
                           -> Redis queue (optional)
~~~

## 顺着一次请求看代码

| 阶段 | 入口 | 不负责什么 |
| --- | --- | --- |
| HTTP 接线 | [gateway](../../../internal/gateway/readme.md) | 不把控制台按钮当权限 |
| 身份 / 授权 | [auth](../../../internal/auth/auth.go)、[authz](../../../internal/authz/decide.go) | 不用客户端 metadata 认定身份 |
| 候选编译 | [router](../../../internal/router/readme_cn.md) | 不扩大模型允许名单 |
| 上游执行 | [dataplane](../../../internal/dataplane/readme_cn.md) | 不在断流后拼接另一上游 |
| 协议登记 / 转换 | [provider](../../../internal/provider/readme_cn.md)、[llm](../../../internal/llm/readme_cn.md) | 目录条目不证明协议可执行 |
| 配置 / 身份 / 账务 | [store](../../../internal/store)、[iam](../../../internal/iam/readme_cn.md) | 当前配置不覆盖历史归属 |

普通推理按鉴权、限额、护栏、选路、执行、记账处理。原生任务走专门生命周期；不要套用聊天缓存或重试规则。

## 验证入口

~~~bash
go test ./internal/router ./internal/dataplane/... -count=1
go test ./internal/gateway -run 'TestVisibilityChain' -count=1
~~~

[运行边界](runtime.md) · [计价](pricing.md) · [测试环境](testing.md)
