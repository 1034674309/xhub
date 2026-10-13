# llm

[English](readme.md)

根目录只提供导航，直接依赖子包。

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

本目录仅组织 credentials、dialogue 和 estimate，没有可导入的根包 API。按需要选择子包：水合连接→解析/编码对话→由数据面发送；估算与实际结算分开。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| credentials.Hydrate | 连接参数快照 |
| dialogue.ParseDialogue / EncodeDialogue | 无网络协议转换 |
| estimate.CountTokens / TokenCost | 准入/工具估算 |

### 接入 demo

在仓库根目录执行以下命令。

```sh
go test ./internal/llm/... -count=1
```

验证子包契约；可复制运行的转换 demo 见 [dialogue](dialogue/readme_cn.md)，凭据 demo 见 [credentials](credentials/readme_cn.md)。

### 错误、状态与副作用

统一对话只支持显式可表达的能力；完整供应商字段走原生入口。不要尝试导入 internal/llm 根目录。

## 源码与验证

| Directory | Responsibility |
|---|---|
| [dialogue](dialogue/readme_cn.md) | 统一对话编解码与严格校验 |
| [credentials](credentials/readme_cn.md) | 可信凭据水合 |
| [estimate](estimate/readme_cn.md) | token 和费用估算 |

依赖方向：厂商 → registry/measurement → contract；deployment → registry/capability/llm/dialogue。dataplane 执行请求，gateway 负责鉴权、持久化和装配。

复用官方 openai-go/v3、anthropic-sdk-go、Google genai 响应类型。SSE 解码位于 dataplane/dialogue。原生 bypass 使用标准 HTTP 保留未知字段，SDK 不接管重试、路由和结算。
