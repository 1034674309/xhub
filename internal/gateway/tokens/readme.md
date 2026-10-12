# tokens: Local token estimates, parameter discovery and transformation records

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Provide mixed-entry authentication, current models, and namespaced storage before mounting. Utilities resolve the counting model and return estimates; transformations still require protocol validation.

| Entry point or resource | Usage and result |
| --- | --- |
| Module(host, store) | Construct a util.Module and install via Mount/Server.Use |
| POST /utils/token_counter | HTTP entry point used in this demo |

### Integration demo

Use this integration helper during startup. The host must implement the required interfaces; reg can be a gateway Server. For a standard ServeMux, adapt Handle as in the [health demo](../health/readme.md). The helper compiles but needs a complete host to execute; the HTTP demo below checks the mounted endpoint.

```go
package demo

import (
	"github.com/sunqirui1987/xhub/internal/gateway/tokens"
	"github.com/sunqirui1987/xhub/internal/util"
)

// MountDemo 演示启动时挂载模块；h 提供真实宿主能力，reg 接收路由，返回无。
// 调用方须先初始化依赖，st 提供真实存储；挂载会修改路由表，不执行 HTTP 请求。
func MountDemo(h tokens.Host, st tokens.Store, reg util.Registrar) { tokens.Module(h, st).Mount(reg) }
```

Start a configured local gateway first. Set XHUB_DEMO_TOKEN to a valid utility credential and configure demo-model. See [wire.go](../wire.go) and [routes.go](../routes.go) for complete host wiring; do not substitute nil dependencies.

Run the following commands from the repository root.

```sh
curl -sS -i -X POST "http://localhost:4000/utils/token_counter" \
  -H "Authorization: Bearer $XHUB_DEMO_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"model":"demo-model","messages":[{"role":"user","content":"Hello"}]}'
```

Returns the count and counting model, depending on deployment/tokenizer configuration; it is not a supplier bill.

### Errors, state, and side effects

Requires valid utility-entry credentials. Unknown models and unconvertible fields must fail. Counting does not issue inference or grant inference authorization.

## Responsibility

Local token estimates, parameter discovery and transformation records. No vendor inference; estimates are not billing measurements; store failures must be observable.

## Review order

tokens.go: Host / Store → Module → tokenCounter / supportedOpenAIParams / transformRequest

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

Host injects auth/model snapshots; Store supplies KV; llm/estimate counts.

## Verification

Adjacent tests: tokens_test.go

From repository root: go test ./internal/gateway/tokens -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
