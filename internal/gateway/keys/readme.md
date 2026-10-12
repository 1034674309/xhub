# keys: Platform and personal API key lifecycle

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Implement Host in startup wiring and mount the key module. Authenticate and authorize each resource before creating, updating, revoking, or rotating keys.

| Entry point or resource | Usage and result |
| --- | --- |
| Module(host) | Construct a util.Module and install via Mount/Server.Use |
| GET /key/list | HTTP entry point used in this demo |

### Integration demo

Use this integration helper during startup. The host must implement the required interfaces; reg can be a gateway Server. For a standard ServeMux, adapt Handle as in the [health demo](../health/readme.md). The helper compiles but needs a complete host to execute; the HTTP demo below checks the mounted endpoint.

```go
package demo

import (
	"github.com/sunqirui1987/xhub/internal/gateway/keys"
	"github.com/sunqirui1987/xhub/internal/util"
)

// MountDemo 演示启动时挂载模块；h 提供真实宿主能力，reg 接收路由，返回无。
// 调用方须先初始化依赖；挂载会修改路由表，不执行 HTTP 请求。
func MountDemo(h keys.Host, reg util.Registrar) { keys.Module(h).Mount(reg) }
```

Start a configured local gateway first. Set XHUB_DEMO_TOKEN to a session token authorized for this endpoint. See [wire.go](../wire.go) and [routes.go](../routes.go) for complete host wiring; do not substitute nil dependencies.

Run the following commands from the repository root.

```sh
curl -sS -i "http://localhost:4000/key/list" \
  -H "Authorization: Bearer $XHUB_DEMO_TOKEN"
```

A visible key list, limited to the authorized scope, without granting access to plaintext secrets.

### Errors, state, and side effects

Creation/rotation may return one-time secrets. Persist hashes and avoid plaintext logs. Revocation/expiry must affect the next authentication, and lists must apply Scope.

## Responsibility

Platform and personal API key lifecycle. No inference; review creation, revocation, expiry, scope and secret disclosure together.

## Review order

host.go → mount.go → generate.go → admin.go / personal.go

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

Host injects identity/storage; auth/IAM own hashes and scope.

## Verification

Adjacent tests: key_input_test.go, personal_test.go

From repository root: go test ./internal/gateway/keys -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
