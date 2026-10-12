# E2E 与路由、会话设计对齐记录

日期：2026-10-12。范围：检查 Python 真实验收、Codex 模拟器、后台 regression 和浏览器 E2E 中涉及会话、部署、模板、组、权重、回退及账单的契约。扫描后选择 37 个相关浏览器场景，分别运行直接落库与专属 Redis 模式；没有运行全部 350 个浏览器场景，也没有重新执行外部付费的 26 项真实验收。

依据：[会话保持与供应商粘性](../../development/zh-CN/session-affinity-design.md)、[路由模板](../../development/zh-CN/routing.md)、[路由组与回退](../../development/zh-CN/routing-groups.md)、[模型与端点](../../development/zh-CN/model-endpoints.md)。

## 结论与修正

旧测试存在与新设计不符的假设，同时发现了一个独立的产品缺陷：Redis 异步账单转换没有复制 `SessionSource`，使来源字段落库后为空。已补齐转换，单元测试覆盖五种来源、旧载荷、成功和失败记录，真实 Redis→数据库及 Codex 浏览器流程验证来源保留。直接落库与 Redis 模式需要分别验收，前者不能证明后者正确。

用户提供的原始错误是“Codex 会话或模型归属错误”。这次发现的来源丢失与错误归属断言都可能导致该检查失败；原运行目录在共享工作区中已变化，不能把此次独立复现直接认定为那一条原始账单的唯一原因。当前没有取得原付费运行重新通过的证据。

| 原有假设或验证缺口 | 新设计要求及处理 |
| --- | --- |
| 使用 `Session_id` 或 `session-id` 头，把明确 ID 的每轮来源都算作 explicit | 使用 `X-Session-Id`；首轮 explicit，合法 `previous_response_id` 续接最终是 parent，即使明确 ID 与父响应相同 |
| 同一供应商就足以证明部署粘性 | 从请求别名、供应商、上游型号及观察器标签唯一匹配稳定部署；冷却使用运行时身份，粘性使用稳定 `DeploymentID` |
| allocations 只列出的部署可以执行 | 未列出的合法部署权重为 1；确定性验收必须显式将其余合法部署置零，不把分配列表当白名单 |
| 空回退表清空默认链 | 省略或 `[]` 继承；源映射到 `[]` 只禁用该源、该类别；空 routing_groups 则不启用组 |
| 成员继承组配置后可以调用任意组成员 | 成员请求只展开自身部署；精确模型规则优先于组策略；组分配不叠加成员默认权重 |
| 每轮都应先失败再回退 | 首次失败后转入备用；成功更新偏好，下一轮优先复用仍合法的成功目标 |
| disable_fallbacks 关闭所有尝试 | 禁止跨模型回退及跨模型偏好复用；安全的池内尝试仍遵守重试与入口重放规则 |
| 回退后账单模型改成备用型号 | 日志保留客户端请求模型或组及会话；实际供应商、部署和上游型号单独核验；五级费用按成功调用结算 |
| 显式 ID 可以跳过父响应校验 | 父响应仍须验证存在、权限、请求范围及 ID 一致；归属固定原部署；失败或未完成流不能发布可续接上下文 |
| 临时候选就是已成功的永久目标 | 临时申请与成功比较更新分开；旧请求迟到成功不能覆盖新目标，失败不能无条件删除其他请求偏好 |
| 配置了 fallback 就必然禁用响应缓存 | 合法响应缓存仍可命中；验证真实路由与计费时保持同一明确会话、逐轮改变正文，并核对真实上游调用和非零账单；相同正文缓存及目标权限复核另由后台测试验证 |
| 多轮失败互不影响 | 真实 Redis 会累积失败并进入冷却。回退类别、错误日志分页测试使用私有模板关闭被动冷却；专用冷却测试仍保留真实冷却验证 |
| 浏览器启动 Redis 即覆盖了异步落库 | 专属 Redis 必须实际连接，启动真实 FlushSpend worker；测试仅缩短等待周期，仍经过队列、数据库事务与确认流程，刷写失败整批保留 |

## 设计与证据对应

| 设计 | 单元验证 | 后台真实边界验证 | 浏览器与验收流程 |
| --- | --- | --- | --- |
| 会话保持与供应商粘性 | 来源优先级、冲突与父响应、模型/端点范围、缓存比较更新、异步账单来源 | Codex 三轮 direct/Redis、增量与原生工具续接、失败流不可续接、并发临时候选与迟到成功、会话路由契约 | Codex 三轮→调用日志→费用；Playground 多轮与刷新、模型及 Chat/Responses 切换同组、清空新会话；会话合并分页 |
| 路由模板 | 权重与精确规则、组规则及默认继承、回退图、策略校验 | 模板完整保存回读、绑定层级、默认与模板优先级、重试次数、多供应商权重、零权重排除、真实冷却 | 配置与诊断、组织/团队/密钥继承、权重删除清理、真实故障后切换并复用 |
| 路由组与回退 | 成员候选限制、组分配、三类错误分类与空值语义 | 组生命周期、三类回退、有序链、禁用与继承、合并环、预算及目标权限复核 | 完整模板组配置→成员/组调用→回退→日志与计费；模型回退编辑与错误提示；桌面/移动端策略保存 |
| 模型与端点 | 稳定部署身份、端点能力、请求别名与实际目标、执行与费用快照 | 部署重复身份、多供应商续接、模型发现路径、禁用目标、协议转换与端点边界 | 按部署行调用计费、端点保存与切换、Chat/Responses/Claude/Gemini/Vertex 入口、无部署恢复、中英文连接失败 |

这些证据按层分工：单元断言确定性细节，后台通过真实路由、隔离数据库和本地 HTTP 上游检查业务结果，浏览器检查页面操作、接口、真实调用及落库。对供应商行为使用受控本地上游，不把本地缓存 token 或网关耗时当成外部缓存实测。

## 修改文件

本工作区另有并行修改。下表列出本次对齐涉及的文件与内容，不将其他 README、响应缓存实现或验收运行器的并行改动算作此次独立交付。

| 文件 | 内容 |
| --- | --- |
| `internal/dataplane/runtime/live.go`、`live_test.go` | Redis 账单保留 SessionSource；来源、请求/实际归属、旧载荷与失败测试 |
| `internal/gateway/browser_fixture_test.go`、新增 `browser_redis_fixture_test.go` | 专属 Redis、实际连接断言、异步刷写、隔离与刷写失败不确认 |
| 新增 `cmd/regression/session_affinity_concurrent_test.go` | 真实 Redis 并发首次请求、A 失败转 B、旧 A 迟到、第三轮 B，以及日志和五级费用 |
| `cmd/regression/codex_agent_test.go`、`responses_native_continuation_test.go` | 正确会话头、parent 来源、续接与实际归属；Codex direct/Redis |
| `cmd/regression/fallback_acceptance_test.go`、`multi_supplier_test.go` | 回退缓存契约与权限复核、稳定部署身份及权重 |
| `cmd/regression/internal_modules_test.go` | 适配当前 Cache.Get 的 Body 字段，继续验证读取快照隔离 |
| `e2e/codex_simulator.py`、`codex_browser.cjs`、`test_codex_agent.py` | 明确 ID 传输、逐轮来源及浏览器观察 |
| `e2e/acceptance/chat.py`、`support.py`、`test_acceptance_codex.py` | 部署唯一匹配、Codex 模型/会话/来源及账单；共享文件保留现有并行改动 |
| `e2e/acceptance/routing.py`、`test_acceptance_routing.py` | 未列部署显式排除、成功偏好复用、同会话不同正文、请求归属与清理 |
| `docs/testdata/real-acceptance/dataset.json`、`e2e/test_acceptance_support.py`、`e2e/acceptance/support.py` | 删除旧“配置回退即禁缓存”要求，清单明确同会话、相同正文及成功响应缓存；校验缺失及错误配置 |
| `e2e/acceptance/runtime.py`、`category_help.py` | 验收分类说明同步为成功目标复用与逐轮真实调用，保留共享文件的其他并行内容 |
| `frontend/e2e/fallback-acceptance-scenario.ts`、`model-fallback-scenario.ts` | 首轮真实回退/后续缓存，独立零费用账单与命中前目标权限复核 |
| `frontend/e2e/routing-groups-scenario.ts`、`playground-workspace-scenario.ts` | 成员/组语义，模型/端点切换与恢复仍归同会话 |
| `frontend/e2e/pagination-scenario.ts` | 正式会话头、会话分页与失败分页、私有模板隔离冷却 |
| 本文 | 四份设计、修正项、测试命令、结果与外部边界 |

## 实际运行结果

结果文件均在仓库根目录 `.e2e/design-alignment/`。Go 数量包含父测试与子测试，不能与独立浏览器场景数直接相加。

| 测试层 | 通过 | 跳过 | 失败 | 最终证据 |
| --- | ---: | ---: | ---: | --- |
| Go 相关单元 | 562 | 2 | 0 | `go-unit-final.jsonl`，18 个包通过 |
| 后台直接落库 | 87 | 2 | 0 | `regression-final.log` |
| 后台专属 Redis | 88 | 0 | 0 | `regression-redis.log` |
| Python 全部测试（启用隔离数据库测试） | 152 | 0 | 0 | `python-final.log` |
| 前端 unit/component | 132 | 0 | 0 | `frontend-tests.log`，17 个文件 |
| 浏览器直接落库 | 37 | 0 | 0 | `browser-direct-verified/results.json`、`junit.xml`、`html/index.html` |
| 浏览器专属 Redis | 37 | 0 | 0 | `browser-redis-verified/results.json`、`junit.xml`、`html/index.html` |
| 专属 Redis 夹具定向验证 | 3 | 0 | 0 | `browser-fixture.log`，父测试及两个子测试 |
| Codex 定向验证 | 3 | 0 | 0 | `codex-final.log`，父测试及 direct/Redis |
| 并发与缓存边界定向验证 | 2 | 0 | 0 | `concurrent.log` |

浏览器共 74 次成功执行，对应 37 个不同场景；`unexpected=0`、`flaky=0`。覆盖报告中的“37/350”表示选择范围，未选的 313 项不是本次失败。早期排查日志中的冷却、响应缓存假设失败已被最后两次 verified 运行替代。

Go 单元跳过的是需要真实付费供应商的 `TestQiniuSeedanceLive`、`TestQiniuFalLive`；直接落库后台的两个跳过项是 Redis 冷却分支，随后专属 Redis 运行没有跳过。

### 命令

以下在仓库根目录执行，前端命令在 `frontend` 执行；长日志重定向到上述最终证据文件。

```sh
go test -count=1 -json ./internal/router ./internal/gateway/affinity \
  ./internal/gateway/models ./internal/gateway/prefs \
  ./internal/dataplane/routing ./internal/dataplane/execution \
  ./internal/dataplane/runtime ./internal/live ./internal/provider/...

bash scripts/regression.sh -v \
  'Test(Codex|SessionAffinity|Fallback|Cooldown|RoutingGroup|RouteTemplate|ModelFallback|ModelWeights|Responses|DialogueAlias|TemplateRouting|MultiSupplier|ModelDiscovery|InternalCacheAndPluginBoundary)'

python3 .e2e/design-alignment/redis_regression.py

XHUB_MEDIA_RECOVERY_DATABASE_TEST=1 \
  python3 -m unittest discover -s e2e -p 'test_*.py' -v

go test -count=1 -v ./internal/gateway -run '^TestBrowserRedisFixtureIsolationAndFlush$'
bash scripts/regression.sh -v 'TestCodexAgentConversation'
bash scripts/regression.sh -v '^Test(SessionAffinityConcurrentLateSuccess|InternalCacheAndPluginBoundary)$'
git diff --check
```

`redis_regression.py` 是本次证据目录内的执行包装器：创建随机端口、无持久化的专属 Redis，将 URL 传给 regression，运行上面的业务匹配范围（不含 InternalCacheAndPluginBoundary），结束终止子进程。它不清空共享缓存。

```sh
# 在 frontend 目录
npx vitest run --project unit --project component \
  src/components/route_templates src/components/routing_groups \
  'src/app/(dashboard)/playground/hooks/useChatHistory.test.ts' \
  'src/app/(dashboard)/playground/hooks/requestScope.test.ts' \
  'src/app/(dashboard)/playground/components/chat_ui/SessionManagement.test.tsx' \
  'src/app/(dashboard)/playground/page.test.tsx'
```

```sh
# 在仓库根目录：Redis 模式；直接落库运行去掉 E2E_REDIS=1，
# 并将 E2E_RUN_DIR 改为 browser-direct-verified。两种模式均实际执行。
E2E_REDIS=1 E2E_ENDPOINT_UNBOUND=1 \
E2E_RUN_DIR=$PWD/.e2e/design-alignment/browser-redis-verified \
E2E_BUILD_DIR=.next-design-redis E2E_UI_PORT=3217 E2E_GW_PORT=4217 E2E_UP_PORT=4218 \
bash scripts/e2e.sh \
  scenarios/legacy/codex-agent scenarios/legacy/fallback-acceptance \
  scenarios/legacy/playground-workspace scenarios/legacy/responses-continuation \
  scenarios/legacy/routing-groups scenarios/legacy/product-fixes \
  scenarios/legacy/model-deployment-acceptance scenarios/legacy/model-endpoints \
  scenarios/legacy/model-fallback scenarios/legacy/route-diagnostic \
  scenarios/legacy/weighted-routing scenarios/legacy/pagination/sc-098.test.ts \
  scenarios/legacy/pagination/sc-099.test.ts scenarios/pages/route-templates.test.ts
```

## 清理与未覆盖边界

浏览器和后台使用私有 PostgreSQL schema；两种浏览器 verified 运行的 `cleanup.log` 保存了本轮 schema 删除记录。新增模板、部署、密钥由场景 finally/defer 删除；私有 Redis 无持久化并随测试结束退出。失败刷写测试只在私有 schema 添加 CHECK 约束，验证整批不落库且队列不被提前确认。未通过 testdata 重置公用数据库或 Redis。

本次检查和修正了上述设计相关的 E2E 契约，没有证明其余 313 个浏览器场景全部通过。付费 26 项真实验收没有重跑，外部供应商的账号、网络、额度、缓存有效期、实际 token 价格和稳定性不在本次自动化结论内。原 Codex 付费失败应以之后独立隔离运行的新日志确认，不能以本地 37 项通过替代。

本地上游验证了正确部署选择、调用次数、协议、错误、账单及数据清理。供应商 prompt 缓存必须读取真实上游 usage 单独验证，不能从网关响应缓存、粘性或响应耗时推断。本设计文档引用的供应商缓存实测属于另一份已有记录，本次没有新增该外部测量。
