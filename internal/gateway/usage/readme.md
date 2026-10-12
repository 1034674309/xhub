# usage: Reports, activity, guard statistics and log reads

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Provide both usage Host and identity.Gate, then constrain log queries with Scope. Individual and session logs still require their own read authorization.

| Entry point or resource | Usage and result |
| --- | --- |
| Module(host) | Construct a util.Module and install via Mount/Server.Use |
| GET /spend/logs/ui | HTTP entry point used in this demo |

### Integration demo

Use this integration helper during startup. The host must implement the required interfaces; reg can be a gateway Server. For a standard ServeMux, adapt Handle as in the [health demo](../health/readme.md). The helper compiles but needs a complete host to execute; the HTTP demo below checks the mounted endpoint.

```go
package demo

import (
	"github.com/sunqirui1987/xhub/internal/gateway/identity"
	"github.com/sunqirui1987/xhub/internal/gateway/usage"
	"github.com/sunqirui1987/xhub/internal/util"
)

// UsageHost 合并模块挂载所需的用量查询与身份授权接口，由生产宿主实现。
type UsageHost interface {
	usage.Host
	identity.Gate
}

// MountDemo 演示启动时挂载模块；h 提供真实宿主能力，reg 接收路由，返回无。
// 调用方须先初始化依赖；挂载会修改路由表，不执行 HTTP 请求。
func MountDemo(h UsageHost, reg util.Registrar) { usage.Module(h).Mount(reg) }
```

Start a configured local gateway first. Set XHUB_DEMO_TOKEN to a session token authorized for this endpoint. See [wire.go](../wire.go) and [routes.go](../routes.go) for complete host wiring; do not substitute nil dependencies.

Run the following commands from the repository root.

```sh
curl -sS -i "http://localhost:4000/spend/logs/ui" \
  -H "Authorization: Bearer $XHUB_DEMO_TOKEN"
```

Authorized paginated call logs, possibly empty. This example creates no usage events.

### Errors, state, and side effects

Spend scopes must not be summed. Prompt visibility depends on settings and authorization. Queries and deletion must enforce trusted scopes.

## Responsibility

Reports, activity, guard statistics and log reads. No spend creation or vendor calls; review tenant authorization before filtering, pagination and amounts.

## Review order

host.go / mount.go → reports.go → activity.go / entity_activity.go → guardrails.go

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

Host supplies scoped identity/storage; IAM/store query persisted data.

## Verification

Adjacent tests: activity_test.go, cost_breakdown_test.go, guardrails_test.go, health_connection_test.go, log_guardrail_test.go, reports_test.go

From repository root: go test ./internal/gateway/usage -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
