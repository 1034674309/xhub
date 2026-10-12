# Gateway architecture

[简体中文](readme_cn.md)

The root composes HTTP and adapts identity, quotas and persistence. Business modules are split by responsibility.

| Package | Responsibility |
|---|---|
| [affinity](affinity/readme.md) | Tenant-scoped affinity, response continuation and task ownership |
| [billing](billing/readme.md) | Deployment rate selection and per-call pricing |
| [compliance](compliance/readme.md) | Compliance-check endpoints for request logs |
| [family](family/readme.md) | Catalog resource endpoints and tenant isolation |
| [guard](guard/readme.md) | Guard configuration, rules and external runners |
| [health](health/readme.md) | Liveness, readiness and console startup configuration |
| [identity](identity/readme.md) | Users, memberships and management permissions |
| [keys](keys/readme.md) | Platform and personal API key lifecycle |
| [models](models/readme.md) | Deployments, credentials, capabilities and routing configuration |
| [prefs](prefs/readme.md) | Preference/route-setting reads, merges and persistence |
| [publichub](publichub/readme.md) | Public model catalog, filtering, sorting and links |
| [requestlog](requestlog/readme.md) | Content encoding, streamed-log reconstruction and secret redaction |
| [templateauth](templateauth/readme.md) | Route-template selection authorization |
| [tokens](tokens/readme.md) | Local token estimates, parameter discovery and transformation records |
| [usage](usage/readme.md) | Reports, activity, guard statistics and log reads |

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Startup runs config.Load → iam.Open/Migrate → store.Open → gateway.New → Server.Run. Embed Handler in an HTTP host and install modules through Use/Handle. Management and inference enforce identity, action authorization, and model access.

| Entry point or resource | Usage and result |
| --- | --- |
| gateway.New(cfg,store,iam) | Assemble initialized real dependencies |
| Server.Handler() / Server.Run(addr) | Provide an HTTP handler or start listening |
| Server.Use(module) / Handle(pattern,handler) | Install modules and dedicated handlers |

### Integration demo

Configure a dedicated PostgreSQL database_url in /tmp/xhub-demo.yaml. See [config](../config/readme.md) for the schema. Startup migrates/synchronizes tables; after shutdown the database owner can remove the dedicated demo database.

Run the following commands from the repository root.

```sh
go run ./cmd/gateway -config /tmp/xhub-demo.yaml -addr :4000

curl -sS -i http://localhost:4000/health/liveness
curl -sS -i http://localhost:4000/health/readiness
```

Run curl in a second terminal. Liveness returns 200 status=ok; readiness returns 200 status=ready when database Ping succeeds and 503 on failure.

### Errors, state, and side effects

gateway.New can connect Redis, start refresh work, and seed an administrator. Use a dedicated database. Probe success does not prove a supplier is callable.

## Root review route

| Files | Review question |
|---|---|
| server.go, routes.go, catalog.go | Which dependencies/routes are composed? Do dedicated handlers take precedence? |
| ingress.go, access.go, session.go, limits.go | Are identity, model access, budgets, rates and failure responses correct? |
| engine.go, wire.go, bypass.go | How are dataplane capabilities, HTTP and runtime state injected? |
| route_settings.go, template_routing.go, route_preview.go | Do templates, previews and actual execution agree? |
| spend.go | Are notes consumed, failed calls free and task settlements idempotently persisted? |
| onboarding.go, theme_settings.go, removed.go | Initialization, theme settings and explicit rejection of removed routes. |

Read Host / Store / DeploymentFinder / Pinger before Module/handlers, then callers. affinity.State owns state; billing and requestlog compute independently of Server. wire implements dataplane/contract and dataplane/runtime.Runtime.

Management ingress enters dedicated modules; inference enters identity/quotas, dataplane/execution and spend/log persistence. See [dataplane](../dataplane/readme.md). Root tests cover HTTP composition and persistence; pure units stay beside implementation.

## Test layers

Units stay beside source; real HTTP/persistence contracts live in cmd/regression; browser-to-gateway-to-local-upstream flows live in frontend/e2e. Do not create test-only integration packages.

From repository root:

    go test ./internal/dataplane/... ./internal/gateway/... ./internal/llm/... ./internal/provider/... -count=1
    bash scripts/regression.sh 'TestComplianceModuleContract|TestUpstreamDiagnosticsContract|TestResponsesFailedStreamCannotContinue' -v
    bash scripts/e2e.sh e2e/playground-transport.spec.ts e2e/responses-continuation.spec.ts e2e/model-fallback.spec.ts

Tests use real local HTTP/PostgreSQL and isolated fixtures, cleaning private schemas without overwriting user data. Paid vendors/external guards need declared credentials; offline tests cover configuration, support, refusal and failure paths.
