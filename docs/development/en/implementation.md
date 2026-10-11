# Source map by task

| Change | Implementation | Verification |
| --- | --- | --- |
| Provider protocol | internal/provider, internal/llm | go test ./internal/provider/... ./internal/llm/... |
| Model save | internal/gateway/models, internal/provider/deployment | model-endpoints E2E |
| Roles and visibility | internal/authz, internal/gateway/identity | permission / visibility_chain tests |
| Virtual keys | internal/gateway/keys, internal/iam/keys.go | keys-playground E2E |
| Routing and fallback | internal/router, internal/dataplane/routing/fallback.go | route_preview / fallback regression |
| Price snapshots | internal/catalog/cost_at.go, internal/gateway/spend.go | pricing regression |
| Settlement deduplication | internal/iam/usage.go, internal/live/redis.go | usage_idempotency / redis tests |
| XGo guardrails | internal/gateway/guard | custom / guard regression |
| UI strings | frontend/src/i18n/messages | Component tests and bilingual E2E |

## Find symbols and tests

~~~bash
rg 'RegisterTransport|RegisterModel' internal/provider
rg 'CostAt|RecordSpend|RecordUsage' internal
rg 'Resolve|Compile' internal/router internal/dataplane
rg 'Test.*Pricing|Test.*Fallback' cmd/regression
~~~

[Architecture](architecture.md) · [Protocol code examples](provider-catalog.md) · [Test commands](testing.md)
