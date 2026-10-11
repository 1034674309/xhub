# 后端模块总览

[English](README.md) · [全功能实现说明](../docs/development/zh-CN/implementation.md)

## 说明

本目录实现 XHub 的身份、管理控制面、模型数据面、路由、计价、持久化。跨模块回归及共用测试夹具集中在 [cmd/regression](../cmd/regression/readme_cn.md)。cmd 负责组合和启动，internal 的包负责可测试的业务行为；Go 的 internal 导入规则限制外部项目直接依赖这些实现。对外稳定边界是 HTTP 协议和配置文档，不是所有导出的 Go 符号。
一次推理先识别会话或虚拟密钥，从数据库恢复组织、团队、项目和所有者，再检查动作、模型范围、预算与限流。控制面解析本次路由模板，数据面选择可用部署、构造协议并发送；成功响应提取 usage、保存价格快照和调用归属，最后提交成功缓存及粘性状态。
身份数据和账务在 iam，框架配置及模型部署在 store，Redis 热状态在 live，响应缓存、运行日志与分页工具在 util。五个费用作用域是同一次消费的视图，不能相加成为总消费。流输出后禁止拼接第二个供应商响应；数据库提交前不能确认队列。

当前目录是后端实现的组织根，没有直接生产 Go 文件或独立 Go 包。子目录按业务职责划分，各级 README 介绍所在目录本身；子包导出的符号供仓库内模块组合使用，不是面向外部项目的 SDK。

## 对外接口

### 进程与 HTTP 边界

生产入口是 [cmd/gateway/main.go](../cmd/gateway/main.go)：读取 -config（默认 configs/config.yaml）和 -addr（默认 :4000），调用 config.Load、iam.Open/Migrate、store.Open、gateway.New，最后 Server.Run 监听请求。配置、数据库连接或迁移失败结束启动；监听失败结束进程。根目录不直接挂载 HTTP 路由，实际接口由 gateway 显式模块、目录接线及 provider 传输登记组成。

Server.Handler 返回宿主和测试使用的 http.Handler；Handle 安装路由，Use 安装模块。管理调用要求会话与动作授权，推理接受可推理会话或虚拟密钥，主密钥只有有限引导和应急权限。字段、认证、状态码见 [API 文档](../docs/api/zh-CN/models.md)及模块 README。路径列在目录中不等于对应业务已经实现。

### 包间协作契约

httpx.Module/Registrar 定义模块安装；auth.Principal 与 authz.Actor/Object/Scope 连接身份、动作授权和查询范围。dataplane.Adapted、Bypass、Runtime 等宿主接口把身份、存储、路由、日志依赖注入数据面；gateway 子模块通过 Host/Gate 调用父模块，避免循环导入。config.ModelEntry、provider.Transport 和 router 策略连接部署配置与实际执行。每个类型的输入、返回值、错误及副作用由拥有它的子包详细说明。

## 内部核心实现

### 启动装配

IAM 连接 PostgreSQL 并迁移身份和账务表，Store 管理框架配置与模型资源。gateway.New 创建授权器、上游 HTTP 客户端、响应缓存、在途计数和扩展注册表；尝试连接配置的 Redis，合并持久设置，加载部署和价格覆盖，启动定时价格刷新并播种初始管理员。显式模块先挂载，目录路由后补齐，重复路径不覆盖先登记处理器。provider/all 的导入触发供应商初始化登记，配置必须声明匹配能力和传输。

### 请求链与失败边界

HTTP 入口识别凭据及会话并确认当前状态，按动作及可信对象归属授权，再检查模型、预算和限流。请求选择密钥→团队→组织链中最近的完整模板或内置模型分配。数据面处理正文、护栏、插件、能力过滤、禁用/冷却候选、排序、缓存和发送；需要时转换对话协议，原厂传输按固定登记动作保留原厂字段。重试、回退和超时不能破坏已输出的流；流开始后不能拼接另一个供应商响应。

异步任务保留创建部署和调用上下文，轮询返回同一部署，成功终态的有效实测用量才结算。未知创建结果不能盲目重放。日志、请求日志和持久审计用途不同，脱敏不能代替读取授权。

### 状态与持久化

IAM 事务保存事件、归属及费率快照，并按请求/任务标识防止重复结算。用户、密钥、团队、项目、组织费用是同一消费的不同视图，不能相加。Store 的 KV 不负责身份授权。Redis 保存热支出、限流、冷却、粘性和队列，消费者先提交数据库再确认；缺失 Redis 的进程状态与同步路径不具备跨实例共享保证。进程响应缓存只保存完整成功字节，键必须包含身份和配置上下文。

## 源码职责与入口

### 子目录索引

| 目录 | 职责 |
| --- | --- |
| [auth](auth/readme_cn.md) | 调用身份解析 |
| [authz](authz/readme_cn.md) | 动作授权与查询范围 |
| [catalog](catalog/readme_cn.md) | 模型目录、费率与用量归一化 |
| [config](config/readme_cn.md) | 启动配置与模型定义 |
| [dataplane](dataplane/readme_cn.md) | 推理执行与结算数据面 |
| [gateway](gateway/readme_cn.md) | HTTP 网关与模块组合 |
| [hooks](hooks/readme_cn.md) | 请求在途计数 |
| [httpx](httpx/readme_cn.md) | HTTP 错误与模块协议 |
| [iam](iam/readme_cn.md) | 身份、授权资源与事务账务 |
| [live](live/readme_cn.md) | Redis 热状态与可靠用量队列 |
| [llm](llm/readme_cn.md) | 上游请求构造与协议适配 |
| [plugin](plugin/readme_cn.md) | 上游调用扩展注册 |
| [provider](provider/readme_cn.md) | 供应商能力与传输登记 |
| [router](router/readme_cn.md) | 部署过滤、排序与加权分流 |
| [store](store/readme_cn.md) | 框架配置与模型存储 |
| [util](util/readme_cn.md) | 响应缓存、运行日志与脱敏、防溢出分页工具 |

### 阅读与验证

启动与 HTTP 从 cmd/gateway/main.go → gateway/server.go、routes.go、engine.go 阅读；推理从 gateway/ingress.go、bypass.go → dataplane 阅读；结算从 iam/schema.sql、usage.go、task_usage.go 阅读。各子目录 README 给出完整本目录文件职责与具体入口。

快速检查命令：go test ./internal/... -count=1。真实后台边界见 [cmd/regression](../cmd/regression/readme_cn.md)，真实浏览器流程使用 cd frontend && npm run test:e2e。数据库验收需检查 XHUB_REGRESSION_STRICT=1 及跳过项；Redis 与真实供应商需要各自环境。单元测试不证明外部供应商连接、付费任务完成或实际账单正确。说明中不存储部署凭据。
