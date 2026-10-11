# llm

根目录只提供导航，直接依赖子包，不保留兼容别名。

| Directory | Responsibility |
|---|---|
| [dialogue](dialogue/readme_cn.md) | 统一对话编解码与严格校验 |
| [credentials](credentials/readme_cn.md) | 可信凭据水合 |
| [estimate](estimate/readme_cn.md) | token 和费用估算 |

依赖方向：厂商 → registry/measurement → contract；deployment → registry/capability/llm/dialogue。dataplane 执行请求，gateway 负责鉴权、持久化和装配。

复用官方 openai-go/v3、anthropic-sdk-go、Google genai 响应类型。SSE 解码位于 dataplane/dialogue。原生 bypass 使用标准 HTTP 保留未知字段，SDK 不接管重试、路由和结算。
