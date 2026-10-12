# identity: Users, memberships and management permissions

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Implement Gate identity, authorization, and storage capabilities at startup. Handlers recover trusted ownership from the database and separately authorize memberships, roles, and template changes.

| Entry point or resource | Usage and result |
| --- | --- |
| Module(gate) | Construct a util.Module and install via Mount/Server.Use |
| GET /user/info | HTTP entry point used in this demo |

### Integration demo

Use this integration helper during startup. The host must implement the required interfaces; reg can be a gateway Server. For a standard ServeMux, adapt Handle as in the [health demo](../health/readme.md). The helper compiles but needs a complete host to execute; the HTTP demo below checks the mounted endpoint.

```go
package demo

import (
	"github.com/sunqirui1987/xhub/internal/gateway/identity"
	"github.com/sunqirui1987/xhub/internal/util"
)

// MountDemo 演示启动时挂载模块；h 提供真实宿主能力，reg 接收路由，返回无。
// 调用方须先初始化依赖；挂载会修改路由表，不执行 HTTP 请求。
func MountDemo(h identity.Gate, reg util.Registrar) { identity.Module(h).Mount(reg) }
```

Start a configured local gateway first. Set XHUB_DEMO_TOKEN to a session token authorized for this endpoint. See [wire.go](../wire.go) and [routes.go](../routes.go) for complete host wiring; do not substitute nil dependencies.

Run the following commands from the repository root.

```sh
curl -sS -i "http://localhost:4000/user/info" \
  -H "Authorization: Bearer $XHUB_DEMO_TOKEN"
```

User information visible to the current identity, requiring a valid session and appropriate permissions.

### Errors, state, and side effects

Membership/role changes affect subsequent live authorization. Validate template bindings with templateauth.Selection and keep writes/audits consistent.

## Responsibility

Users, memberships and management permissions. No inference forwarding; review gates before memberships, templates and mutations.

## Review order

gate.go → mount.go → handlers.go / members.go / route_template.go

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

auth/IAM supply identity and memberships; boundaries inject storage/administration.

## Verification

Adjacent tests: pagination_test.go, route_template_error_test.go

From repository root: go test ./internal/gateway/identity -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
