# 数据面架构

[English](readme.md)

根包只保留目录说明。一次推理或原生任务请求的具体职责如下。

## 如何由其他模块接入

这里的“对外”指仓库内其他 Go 包。Go 的 `internal` 规则限制了仓库外的直接导入；HTTP 客户端使用已挂载的网关接口。

本目录没有根包入口；网关实现 contract 后调用 execution，执行器协调 routing、dialogue/transport、stream、diagnostics、usage 和 runtime。单个工具包 demo 不能替代整条请求链。

| 入口或资源 | 调用方式与返回结果 |
| --- | --- |
| contract.Adapted / Bypass | 宿主依赖接口 |
| execution.Serve / ServeBypass | 完整请求生命周期入口 |
| runtime.Flush | 数据库提交后确认用量队列 |

### 接入 demo

在仓库根目录执行以下命令。

```sh
go test ./internal/dataplane/... -count=1
```

运行数据面子包测试。HTTP 请求 demo 见 [execution](execution/readme_cn.md)，离线续接正文 demo 见 [dialogue](dialogue/readme_cn.md)。

### 错误、状态与副作用

鉴权与配额必须在出站前完成，缓存命中和上游成功是不同事实。适配对话流只有验证可结束终态后才保存续接历史；Responses 在写终态事件之前保存，若随后客户端写出失败，已保存历史不会回滚。原字节透传链的终态校验边界见 stream。真实供应商可用性需单独环境验证。

## 源码与验证

| 目录 | 职责 |
|---|---|
| [contract](contract/readme_cn.md) | 执行器能力接口和调用事实 |
| [diagnostics](diagnostics/readme_cn.md) | 上游原始事实与安全诊断日志 |
| [dialogue](dialogue/readme_cn.md) | 统一对话转换、续接与适配 SSE |
| [execution](execution/readme_cn.md) | 适配与原生请求的生命周期编排 |
| [routing](routing/readme_cn.md) | 候选筛选、模型回退与路由诊断 |
| [runtime](runtime/readme_cn.md) | 运行时快照、冷却、延迟与用量刷新 |
| [stream](stream/readme_cn.md) | 原始流转发与只读用量观察 |
| [transport](transport/readme_cn.md) | 原生 HTTP、正文、路径、鉴权与响应透传 |
| [usage](usage/readme_cn.md) | 用量归一、缺失统计估算与首字时间 |

## 请求关系与 review 顺序

1. gateway/ingress.go、limits.go、bypass.go：入口身份和配额。
2. contract：消费方能力接口；execution/serve.go 或 official.go：请求生命周期。
3. routing：候选和回退；dialogue：适配；transport：原生透传；stream：不转换的流。
4. usage、diagnostics：记录事实；gateway/billing、requestlog：准备结算与日志；spend.go：持久化。
5. runtime：独立维护快照、冷却、延迟和异步刷新。

execution 不依赖 gateway.Server；gateway 是组合入口。contract 当前复用 prefs.RouteSettings 值类型，不加载 prefs 处理器。

## 官方 SDK 与透传边界

- [OpenAI Go](https://github.com/openai/openai-go)：github.com/openai/openai-go/v3 v3.74.0，用于 Chat Completions、Responses 类型和 SSE 解码，兼容项目 Go 1.25。升级前检查 Go 版本要求。
- [Anthropic Go](https://github.com/anthropics/anthropic-sdk-go)：v1.80.0，用于 Messages 响应类型。
- [Google Gen AI Go](https://github.com/googleapis/go-genai)：google.golang.org/genai v1.73.0，用于 Gemini/Vertex 响应类型。

internal/llm/dialogue 使用官方类型解析响应，dialogue/PipeDialogue 使用官方 SSE framing。SDK 可能宽松处理字段类型，适配层仍检查文本、工具与终态；错误类型和不可表达的能力必须失败。原始用量保留，以区分缺失与显式零。

SDK 是调用客户端，不提供完整网关 bypass、跨协议转换、配额、路由、日志和结算。transport 保留 Go 标准库 net/http 的薄转发层，以保留供应商扩展字段、原始状态与错误、multipart 和 SSE 字节。跨协议转换仍由项目承担。目前不实例化 SDK 请求客户端，避免隐式重试与网关重试叠加；原生流观察和日志重建不会用 SDK 重编码客户端响应。

## 测试分层

单元测试与源码同目录；真实 HTTP/持久化契约位于 cmd/regression，浏览器到网关及本地上游流程位于 frontend/e2e。不新增只有测试的 integration 包。

仓库根目录命令：

    go test ./internal/dataplane/... ./internal/gateway/... ./internal/llm/... ./internal/provider/... -count=1
    bash scripts/regression.sh 'TestComplianceModuleContract|TestUpstreamDiagnosticsContract|TestResponsesFailedStreamCannotContinue' -v
    bash scripts/e2e.sh e2e/playground-transport.spec.ts e2e/responses-continuation.spec.ts e2e/model-fallback.spec.ts

测试用真实本地 HTTP、PostgreSQL 与隔离夹具，结束清理私有 schema，不覆盖用户数据。线上付费供应商和外部护栏需单独声明凭据；离线测试验证配置、支持状态、拒绝和失败路径。
