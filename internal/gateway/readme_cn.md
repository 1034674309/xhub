# Gateway 架构

根包是 HTTP 组合根以及身份、配额、存储的适配器。业务模块按职责拆分。

| 目录 | 职责 |
|---|---|
| [affinity](affinity/readme_cn.md) | 租户隔离的会话、响应续接和任务归属 |
| [billing](billing/readme_cn.md) | 部署费率与一次调用费用计算 |
| [compliance](compliance/readme_cn.md) | 请求日志合规检查接口 |
| [family](family/readme_cn.md) | 目录资源接口与租户隔离 |
| [guard](guard/readme_cn.md) | 护栏配置、规则与外部执行器 |
| [health](health/readme_cn.md) | 存活、就绪与控制台启动配置 |
| [identity](identity/readme_cn.md) | 用户、团队成员与管理权限 |
| [keys](keys/readme_cn.md) | 平台与个人 API 密钥生命周期 |
| [models](models/readme_cn.md) | 部署、凭据、端点能力与路由配置管理 |
| [prefs](prefs/readme_cn.md) | 偏好与路由设置的读取、合并和保存 |
| [publichub](publichub/readme_cn.md) | 公开模型目录、筛选排序与链接 |
| [requestlog](requestlog/readme_cn.md) | 内容编码、流式日志重建与秘密脱敏 |
| [templateauth](templateauth/readme_cn.md) | 路由模板选择权限 |
| [tokens](tokens/readme_cn.md) | 本地 token 估算、参数发现与转换记录 |
| [usage](usage/readme_cn.md) | 报表、活动、护栏统计与日志读取 |

## 根包 review 路线

| 文件 | 审核问题 |
|---|---|
| server.go、routes.go、catalog.go | 组合哪些依赖与路由？专门模块能否优先处理对应接口？ |
| ingress.go、access.go、session.go、limits.go | 身份、模型权限、预算、速率与失败响应是否正确？ |
| engine.go、wire.go、bypass.go | 怎样向 dataplane 注入能力、HTTP 和运行时状态？ |
| route_settings.go、template_routing.go、route_preview.go | 模板、预览与真实执行是否一致？ |
| spend.go | 备注消费、失败零扣费、任务幂等结算与落库是否正确？ |
| onboarding.go、theme_settings.go、removed.go | 初始化、主题配置与已移除路由的明确拒绝。 |

先读 Host / Store / DeploymentFinder / Pinger，再读 Module 与处理器，最后看调用方。affinity.State 独立持有状态，billing 和 requestlog 独立计算，不依赖 Server。wire 实现 dataplane/contract 和 dataplane/runtime.Runtime。

管理入口进入专门模块；推理入口经过身份和配额进入 dataplane/execution，随后账单与日志落库。完整关系见 [dataplane](../dataplane/readme_cn.md)。根目录测试负责 HTTP 组合与持久化；纯函数测试留在实现旁边。

## 测试分层

单元测试与源码同目录；真实 HTTP/持久化契约位于 cmd/regression，浏览器到网关及本地上游流程位于 frontend/e2e。不新增只有测试的 integration 包。

仓库根目录命令：

    go test ./internal/dataplane/... ./internal/gateway/... ./internal/llm/... ./internal/provider/... -count=1
    bash scripts/regression.sh 'TestComplianceModuleContract|TestUpstreamDiagnosticsContract|TestResponsesFailedStreamCannotContinue' -v
    bash scripts/e2e.sh e2e/playground-transport.spec.ts e2e/responses-continuation.spec.ts e2e/model-fallback.spec.ts

测试用真实本地 HTTP、PostgreSQL 与隔离夹具，结束清理私有 schema，不覆盖用户数据。线上付费供应商和外部护栏需单独声明凭据；离线测试验证配置、支持状态、拒绝和失败路径。
