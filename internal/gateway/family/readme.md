# family: Catalog resource endpoints and tenant isolation

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Provide identity, scoped storage, and data-plane dispatch through Host. Module mounts only POST /v1/responses and POST /responses. gateway/catalog.go separately uses ServeDataPlane, ServeMixed, and ServeManagement for catalog fallback routes.

| Entry point or resource | Usage and result |
| --- | --- |
| Module(host) | Construct a util.Module and install via Mount/Server.Use |
| POST /v1/responses | HTTP entry point used in this demo |

### Integration demo

Use this integration helper during startup. The host must implement the required interfaces; reg can be a gateway Server. For a standard ServeMux, adapt Handle as in the [health demo](../health/readme.md). The helper compiles but needs a complete host to execute; the HTTP demo below checks the mounted endpoint.

```go
package demo

import (
	"github.com/sunqirui1987/xhub/internal/gateway/family"
	"github.com/sunqirui1987/xhub/internal/util"
)

// MountDemo 演示启动时挂载模块；h 提供真实宿主能力，reg 接收路由，返回无。
// 调用方须先初始化依赖；挂载会修改路由表，不执行 HTTP 请求。
func MountDemo(h family.Host, reg util.Registrar) { family.Module(h).Mount(reg) }
```

Start a configured local gateway first. Set XHUB_DEMO_TOKEN to a session token or virtual key authorized to call the endpoint's model. See [wire.go](../wire.go) and [routes.go](../routes.go) for complete host wiring; do not substitute nil dependencies.

Run the following commands from the repository root.

```sh
curl -sS -i "http://localhost:4000/v1/responses" \
  -H "Authorization: Bearer $XHUB_DEMO_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"model":"demo-model","input":"Hello","stream":false}'
```

Point demo-model at a local fixture upstream supporting responses and supply an authorized credential. A successful request returns a response object; inspect error on failure. This example issues inference.

### Errors, state, and side effects

Catalog paths do not imply generic handlers implement full vendor semantics. Persist resources with trusted owners, not client-supplied ownership.

## Responsibility

Catalog resource endpoints and tenant isolation. Specialized endpoints belong to dedicated modules; review isolation before generic outputs.

## Review order

host.go → mount.go → handlers.go

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

Host supplies identity, scoped storage and catalog metadata; gateway/catalog.go registers routes.

## Verification

Adjacent tests: pagination_test.go, resource_isolation_test.go

From repository root: go test ./internal/gateway/family -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
