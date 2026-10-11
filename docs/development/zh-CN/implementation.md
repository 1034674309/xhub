# 按任务找源码

| 修改任务 | 实现 | 验证 |
| --- | --- | --- |
| 供应商协议 | internal/provider、internal/llm | go test ./internal/provider/... ./internal/llm/... |
| 模型保存 | internal/gateway/models、internal/provider/deployment | model-endpoints E2E |
| 角色与列表 | internal/authz、internal/gateway/identity | permission / visibility_chain 测试 |
| 虚拟密钥 | internal/gateway/keys、internal/iam/keys.go | keys-playground E2E |
| 选路与回退 | internal/router、internal/dataplane/routing/fallback.go | route_preview / fallback regression |
| 计费快照 | internal/catalog/cost_at.go、internal/gateway/spend.go | pricing regression |
| 入账去重 | internal/iam/usage.go、internal/live/redis.go | usage_idempotency / redis 测试 |
| XGo 护栏 | internal/gateway/guard | custom / guard regression |
| 页面文案 | frontend/src/i18n/messages | 组件测试 + 双语 E2E |

## 从符号追到测试

~~~bash
rg 'RegisterTransport|RegisterModel' internal/provider
rg 'CostAt|RecordSpend|RecordUsage' internal
rg 'Resolve|Compile' internal/router internal/dataplane
rg 'Test.*Pricing|Test.*Fallback' cmd/regression
~~~

[架构](architecture.md) · [新协议代码示例](provider-catalog.md) · [验证命令](testing.md)
