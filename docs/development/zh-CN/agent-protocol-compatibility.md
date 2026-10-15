# Agent 协议兼容对照与验收（Issue #9）

本轮范围是 GPT、Grok、DeepSeek 的文本、函数工具、公开/加密推理及 Responses / Chat Completions；图像、音视频及其他厂商本轮不认证。模型名称不是协议能力声明，供应商目录中的模型也不等于当前 Key 有可用渠道。

## LiteLLM 的做法与本次复用

参考官方 LiteLLM `e889d9d6533ac30e776aa3b3780f99fa73e07460`，并对照公司旧库 `f09e043`（基于 v1.95.40）。Python 实现没有直接成为 Go 服务依赖；本次手工移植相关映射规则，保留 MIT 授权，见 [来源与授权](../../../third_party/litellm/NOTICE.md)。这不意味着 XHub 自动拥有 LiteLLM 的全部适配器。

| 处理点 | LiteLLM 参考实现 | XHub 本轮处理 |
| --- | --- | --- |
| 原生与桥接分层 | `responses/main.py` 优先原生配置，否则进入 Completion 桥接 | 在现有统一执行链内区分同协议保留与跨协议投影；复用部署协议登记、HTTP 构造、SDK SSE 解码器 |
| 原生 Responses | OpenAI / xAI 的 Responses 适配器 | 保留 `reasoning`、`include`、`text`、缓存键、输出项、密文和原生 SSE 扩展字段；仅公开模型名被重写 |
| 跨协议参数 | `litellm_completion_transformation/transformation.py` | `reasoning.effort` ↔ `reasoning_effort`，`text.format` ↔ `response_format`，保留函数调用 ID / JSON 参数及公开推理文本 |
| DeepSeek 工具续轮 | `llms/deepseek/chat/transformation.py` 保留 `reasoning_content` | 原生 Chat 全量保留；Chat ↔ Responses 用 `summary_text` 表达公开推理并在工具续轮还原，不伪造空格推理 |
| 未知或无法表达的能力 | 支持参数表及可选 `drop_params` | 同协议扩展交上游验证；跨协议未实现的能力在选路前明确拒绝，不静默删除 |
| 状态与治理 | 适配器之外的 Proxy / Router 链 | 保留现有身份、额度、护栏、路由、日志和计费；没有把标准入口改接绕过统一护栏的 bypass 链 |

当前 DeepSeek 官方协议已比所参考的旧适配器丰富。例如新版文档提供原生 Responses 和 `low/high/max` effort 映射；旧 LiteLLM DeepSeek 代码仍有将 effort 归为 thinking 开/关的逻辑。本轮保留原生值，由已配置供应商验证，不能为了“照搬成熟代码”抹掉新协议能力。

## 已实现的边界

| 能力 | Responses → 原生 Responses | Chat → 原生 Chat | Responses ↔ Chat |
| --- | --- | --- | --- |
| 文本、函数工具、工具结果 | 支持 | 支持 | 支持 |
| JSON / SSE 与实测 usage | 支持，保留原生事件 | 支持，保留 `reasoning_content` 增量 | 转换共享文本/工具/公开推理事件 |
| 推理 effort | 原样保留，合法值由模型决定 | 原样保留 | 映射字段；不会替模型接受不支持的值 |
| 推理摘要 / 加密项 | 原生保留 | 公开推理保留 | 公开文本可转换；加密历史及 `include` 等不能等价表达时拒绝 |
| JSON Schema | 原生保留 | 原生保留 | 转换格式结构；最终能力仍由上游决定 |
| Codex custom / namespace 工具、`additional_tools` | 原生保留 | 本轮不认证 | 拒绝不能表达的工具 / 输入项 |
| 拒绝响应（refusal） | 原生保留，允许原生续轮 | 原生保留 | 明确拒绝未实现的转换，不能生成空白成功 |
| 多候选 `n > 1` / `background: true` | 外发前拒绝 | 多候选外发前拒绝 | 不支持 |
| 图片、音视频、其他原生工具服务 | 本轮不认证 | 本轮不认证 | 本轮不认证 |

标准端点仍为 `/v1/responses`、`/v1/chat/completions`，员工使用 XHub 虚拟密钥和公开模型别名。部署中的 `bypass_openai_responses` / `bypass_openai_chat` 是现有**上游执行方式名称**，不表示员工必须使用 `/bypass/...`；该对外路径仍需单独启用。

`store` / `previous_response_id` 沿用 XHub 已有的受控临时历史机制：上游 `store=false`，网关保存约一小时的身份、模型、入口隔离历史；本轮把原生输出项和密文也纳入其中。它不是 OpenAI 长期 Response 存储，不承诺 GET / DELETE / 后台 cancel 生命周期。HTTP 客户端取消会取消上游请求；缺终态或失败流不能发布成功续接历史。`store=false` 不等于网关完全不保留临时历史。

提交完整加密历史时，也必须具有本身份已知的会话部署归属；不能因权重、冷却、目标禁用或共享缓存故障切换供应商连接。无法确认归属时应发起新会话。

Claude Code 使用 `/v1/messages`，并非 Responses 客户端。公开推理现在按 LiteLLM 的规则转换为无厂商签名的 `thinking` 块，工具续轮还原为 Chat 的 `reasoning_content` 或 Responses 的独立推理摘要项。流式 thinking / text 块依次关闭，交错工具参数在完成校验后逐个发送，避免向已关闭块追加内容。Codex 历史中的 `encrypted_content: null` 仅是空占位，可以投影；非空密文、真实厂商签名和 redacted thinking 仍不能跨此 Messages 桥接。本轮额外验证现有 Messages 桥接：文本、函数工具、缓存提示、客户端归属提示、adaptive effort，以及 `clear_thinking_20251015 + keep=all`（保持历史、不执行清理）。缓存提示不保证跨供应商命中，`metadata.user_id` 不替代网关身份。手工 thinking budget、其他清理规则、未知 safeguards / 输出格式及带签名推理不属于本轮跨协议能力，会明确拒绝。真实 2.1.300 CLI 对不支持的 safeguards 会自行重试不带该字段的请求。

## 真实供应商验收：2026-10-15

使用已授权的 Fenno 连接，在**本地隔离的新网关与临时数据库**执行，没有改生产模型、组织、余额或容器。四项指 Responses JSON / SSE、Chat JSON / SSE；每次限制输出并验证成功日志归属。通过只证明该时点该 Key 的这些请求，不代表所有参数或未来版本均兼容。

| 公司当前模型 ID | 四项结果 | 结论 |
| --- | --- | --- |
| `gpt-5.5` | 0 / 4 | 上游 503；直连 Responses / Chat 同样 503 |
| `gpt-5.6-terra` | 4 / 4 | 通过 |
| `gpt-5.6-sol` | 4 / 4 | 通过；另完成真实 Codex / Claude Code 文件工具往返 |
| `codex-auto-review` | 4 / 4 | 通过；这是供应商提供的 ID，不能视为官方独立型号认证 |
| `gpt-6-astra` | 4 / 4 | 通过 |
| `gpt-6-sol` | 4 / 4 | 通过 |
| `gpt-6.1-sol` | 4 / 4 | 通过 |
| `grok-4.5` | 4 / 4 | 通过 |
| `grok-4.6` | 4 / 4 | 通过 |
| `deepseek-v4-flash` | 0 / 4 | 上游 503；两个直连接口同样失败，尚未完成真实模型验收 |
| `deepseek-v4-pro` | 0 / 4 | 同上 |
| `deepseek-v4-1-flash` | 0 / 4 | 同上；保留供应商原始拼写，不自行改为官方别名 |
| `gpt-image-2` | 未执行 | 图像留到下一轮 |

共 32 / 48 通过，16 项收到上游 503（网关按现有错误契约返回 502）；8 次直连对照同样 503。DeepSeek 的请求、公开推理、流和工具续轮已有离线契约覆盖，但不能标为真实供应商验收通过。

实际 CLI：Codex `0.162.0-alpha.17.2` 与 Claude Code `2.1.300` 均通过新网关调用真实 `gpt-5.6-sol`，各完成读取临时文件 → 传回工具结果 → 最终回答的完整往返，并核对成功账单。发布用固定版本 Codex `0.162.1` / Claude Code `2.1.300` 另在本地协议夹具上实际运行通过，不是模拟 CLI；既有 simulator 测试继续保留。

追加 Messages 修复验收：本机持久测试实例使用同一 Fenno 连接，`gpt-5.6-sol` / `grok-4.6` 各通过原生 Responses 和原生 Chat 两种部署，分别测试 Responses、Chat、Messages 的 JSON/SSE，共 24/24 通过。固定版本 Codex 0.162.1 / Claude Code 2.1.300 分别调用这两个真实模型完成文件工具往返，共 4/4 通过；此前 Grok → Messages 的公开推理错误已复现并修复。以上不包含所有 Grok 型号或参数的认证。

补充完整 DeepSeek ID `deepseek/deepseek-v4.1-flash`、`deepseek/deepseek-v4-pro-0813`、`deepseek/deepseek-v4-flash-20260731` 的 12 项探测仍全部收到上游 503，6 次直连同样失败；本轮暂缓其真实供应商验收。

固定离线夹具只下发读取测试文件的命令，协议验收不依赖宿主只读沙箱实现；真实供应商测试仍使用只读沙箱。CLI 重试有上限，超时清理整个进程组，避免 npm 启动器留下 Rust / 工具宿主子进程。Windows CLI 进程组验收本轮未覆盖。

## 回归与复现

- 单元：参数映射、非法类型、加密/公开推理、原生响应、拒绝/截断流、历史与归属隔离。
- 后台：真实 HTTP / PostgreSQL 覆盖标准端点、护栏脱敏、未授权、预算、JSON/SSE、工具续轮、跨身份密文拒绝、原部署禁用后不得回退、取消、账单和清理。
- 浏览器：另有 Messages 专属用例覆盖两种 OpenAI 上游的公开推理、JSON/SSE 工具续轮和日志计费；管理面保存模型 → 调试台实际调用 → 浏览器原生参数 / 流 / 工具续轮 → Chat 桥接 → 日志与计费 → 删除测试部署和凭据；并回归原有 SSE / JSON 三轮续接及 Codex simulator 日志闭环。
- CI：固定 CLI 版本进入 backend regression；新增浏览器场景进入发布校验，失败也上传 CLI 输出和测试报告。

```bash
# 前置：本机 PostgreSQL 与 redis-server；所有数据库数据使用专属 schema 并回收。
go test -p 2 ./internal/... -count=1 -json
XHUB_TEST_CODEX_BIN=/absolute/path/to/codex \
XHUB_TEST_CLAUDE_BIN=/absolute/path/to/claude \
XHUB_AGENT_REPORT_DIR=/absolute/path/to/reports bash scripts/regression.sh -v
E2E_REDIS=1 E2E_PORTABLE_CONSOLE=1 bash scripts/e2e.sh \
  agent-native-protocol.test.ts messages-reasoning.test.ts scenarios/legacy/responses-continuation scenarios/legacy/codex-agent
node --test scripts/test_release_workflow.cjs
```

实际供应商测试必须显式设置 `XHUB_AGENT_LIVE=1`、`XHUB_AGENT_LIVE_KEY`、`XHUB_AGENT_LIVE_BASE`（以 `/v1` 结束），模型矩阵还可指定逗号分隔的 `XHUB_AGENT_LIVE_MODELS`（最多 12 个），再运行 `go test ./cmd/regression -run '^Test(LiveTextProtocolMatrix|RealAgentClients)$' -count=1`。真实 CLI 仍需上述二进制路径。此模式会产生供应商费用；CI 不设置这些凭据。不要把密钥放进代码或报告。

参考：[LiteLLM Responses](https://docs.litellm.ai/docs/response_api)、[官方推理指南](https://developers.openai.com/api/docs/guides/reasoning)、[DeepSeek Thinking Mode](https://api-docs.deepseek.com/guides/thinking_mode/)、[Claude Code 网关](https://code.claude.com/docs/en/llm-gateway)。适配器的固定源码版本见授权目录。
