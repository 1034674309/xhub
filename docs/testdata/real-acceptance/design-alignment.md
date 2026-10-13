# 路由、会话与账单验收契约

依据：[会话保持与供应商粘性](../../development/zh-CN/session-affinity-design.md)、[路由模板](../../development/zh-CN/routing.md)、[路由组与回退](../../development/zh-CN/routing-groups.md)、[模型与端点](../../development/zh-CN/model-endpoints.md)。测试须以当前注册入口、执行策略和持久化字段为准。

| 范围 | 验证的当前行为 |
| --- | --- |
| 会话身份 | 使用 X-Session-Id；合法 previous_response_id 续接来源为 parent，明确 ID 不能跳过父响应归属与一致性校验 |
| 部署粘性 | 核对稳定 deployment_id、供应商、上游型号及实际调用次数；成功切换后下一轮复用仍合法的新目标 |
| 权重 | 未列出的合法部署权重为 1；需要排除的部署必须显式置零 |
| 路由规则 | 精确模型规则优先于组策略；成员请求只展开自身部署，组分配不叠加成员默认权重 |
| 回退 | 省略或空表继承默认；源映射到空列表只禁用该源类别；disable_fallbacks 禁止跨模型回退及跨模型偏好复用 |
| 并发 | 临时申请与成功比较更新分开；迟到成功不能覆盖已切换目标，失败不能无条件删除他人偏好 |
| 响应缓存 | 相同正文可命中有效缓存；缓存命中仍复核目标权限。验证真实路由时逐轮改变正文并核对上游调用 |
| 日志和费用 | 日志保留请求模型或组；实际部署和上游型号独立记录。五级费用是同一调用的不同归属维度，不相加 |
| Redis 落库 | 实际连接专属 Redis，运行 FlushSpend，经队列、数据库事务与确认流程保留 session_source；刷写失败不确认队列 |
| 续接与流 | 响应归属固定创建时部署；失败或未完成流不能发布成功续接上下文 |

## 执行与证据

单元测试验证确定性分支；后台 regression 使用真实路由、私有 schema 与受控上游；浏览器 E2E 验证配置、调用、日志与费用。直接落库与 Redis 异步落库分别运行。

~~~sh
go test ./internal/router ./internal/gateway/affinity ./internal/dataplane/runtime ./internal/live
bash scripts/regression.sh -v 'Test(Codex|SessionAffinity|Fallback|RoutingGroup|RouteTemplate|ModelWeights|Responses)'
E2E_REDIS=1 bash scripts/e2e.sh scenarios/legacy/codex-agent scenarios/legacy/fallback-acceptance scenarios/legacy/routing-groups scenarios/legacy/responses-continuation
~~~

结果以本轮 results.json、junit.xml、scenario-coverage.json 和后台日志为准，不把固定日期的通过数量当作当前验证结果。每轮保留独立证据目录；测试仅删除自建部署、模板、密钥与私有 schema，专属 Redis 随进程退出。不要重置用户共享数据。

供应商账号、网络、额度、缓存有效期、实际费率与外部稳定性须另行验证。prompt 缓存命中必须读取真实上游 usage，不能从网关响应缓存或耗时推断。
