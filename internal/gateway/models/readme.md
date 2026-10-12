# models: Deployments, credentials, capabilities and routing configuration

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Inject deployment storage, authorization, locks, and probes. Validate declarations before persisting deployments; discovery exposes executable endpoints visible to the caller.

| Entry point or resource | Usage and result |
| --- | --- |
| Module(host) | Construct a util.Module and install via Mount/Server.Use |
| GET /v2/model/info | HTTP entry point used in this demo |

### Integration demo

Use this integration helper during startup. The host must implement the required interfaces; reg can be a gateway Server. For a standard ServeMux, adapt Handle as in the [health demo](../health/readme.md). The helper compiles but needs a complete host to execute; the HTTP demo below checks the mounted endpoint.

```go
package demo

import (
	"github.com/sunqirui1987/xhub/internal/gateway/models"
	"github.com/sunqirui1987/xhub/internal/util"
)

// MountDemo 演示启动时挂载模块；h 提供真实宿主能力，reg 接收路由，返回无。
// 调用方须先初始化依赖；挂载会修改路由表，不执行 HTTP 请求。
func MountDemo(h models.Host, reg util.Registrar) { models.Module(h).Mount(reg) }
```

Start a configured local gateway first. Set XHUB_DEMO_TOKEN to a session token authorized for this endpoint. See [wire.go](../wire.go) and [routes.go](../routes.go) for complete host wiring; do not substitute nil dependencies.

Run the following commands from the repository root.

```sh
curl -sS -i "http://localhost:4000/v2/model/info" \
  -H "Authorization: Bearer $XHUB_DEMO_TOKEN"
```

The deployment management view, containing deployment/endpoint information when the session is authorized.

### Errors, state, and side effects

transport and endpoint_types must be compatible. Deletion must clean dangling weight/fallback references. Probes can contact vendors and incur charges.

## Responsibility

Deployments, credentials, capabilities and routing configuration. No per-request inference; review validation before writes, discovery and deletion cleanup.

## Review order

host.go / mount.go → admin.go / validate.go → available.go / discovery.go → groups.go / fallback.go / routing_groups.go → price.go / schedule.go

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

config/provider declare capabilities; catalog supplies prices; Host injects storage/auth/probes.

## Verification

Adjacent tests: admin_test.go, available_test.go, builtin_test.go, discovery_test.go, fallback_test.go, groups_test.go, list_test.go, price_selection_test.go, price_test.go, routing_groups_test.go, schedule_test.go, validate_credential_test.go

From repository root: go test ./internal/gateway/models -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
