# provider

根目录只提供导航，直接依赖子包，不保留兼容别名。

| Directory | Responsibility |
|---|---|
| [contract](contract/readme_cn.md) | 共享数据契约 |
| [capability](capability/readme_cn.md) | 公开端点与协议匹配 |
| [registry](registry/readme_cn.md) | 内置注册与路径匹配 |
| [deployment](deployment/readme_cn.md) | 显式绑定、校验与候选筛选 |
| [measurement](measurement/readme_cn.md) | 响应/SSE 用量和任务计量事实 |
| [openai](openai/readme_cn.md) | OpenAI 原生协议注册 |
| [anthropic](anthropic/readme_cn.md) | Anthropic Messages 注册 |
| [google](google/readme_cn.md) | Gemini 和 Vertex 注册 |
| [qiniu](qiniu/readme_cn.md) | 七牛任务与 FAL 注册 |
| [volcengine](volcengine/readme_cn.md) | 方舟任务注册 |
| [all](all/readme_cn.md) | 启动时装配全部供应商 |

依赖方向：厂商 → registry/measurement → contract；deployment → registry/capability/llm/dialogue。dataplane 执行请求，gateway 负责鉴权、持久化和装配。
