# guard: Guard configuration, rules and external runners

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Mount Module for management debugging. Real inference calls Evaluate/PreCall before upstream sending, checks blocked/findings, and uses the masked body.

| Entry point or resource | Usage and result |
| --- | --- |
| Module(host) | Construct a util.Module and install via Mount/Server.Use |
| POST /apply_guardrail | HTTP entry point used in this demo |

### Integration demo

Use this integration helper during startup. The host must implement the required interfaces; reg can be a gateway Server. For a standard ServeMux, adapt Handle as in the [health demo](../health/readme.md). The helper compiles but needs a complete host to execute; the HTTP demo below checks the mounted endpoint.

```go
package demo

import (
	"github.com/sunqirui1987/xhub/internal/gateway/guard"
	"github.com/sunqirui1987/xhub/internal/util"
)

// MountDemo 演示启动时挂载模块；h 提供真实宿主能力，reg 接收路由，返回无。
// 调用方须先初始化依赖；挂载会修改路由表，不执行 HTTP 请求。
func MountDemo(h guard.Host, reg util.Registrar) { guard.Module(h).Mount(reg) }
```

Start a configured local gateway first. Set XHUB_DEMO_TOKEN to a session token authorized for this endpoint. See [wire.go](../wire.go) and [routes.go](../routes.go) for complete host wiring; do not substitute nil dependencies. Save a local redact rule named demo-secrets through management, with blocked_words set to ["secret"]. This request tests that rule without enabling default execution.

Run the following commands from the repository root.

```sh
curl -sS -i -X POST "http://localhost:4000/apply_guardrail" \
  -H "Authorization: Bearer $XHUB_DEMO_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"guardrail_name":"demo-secrets","text":"Hello secret"}'
```

With the specified rule configured, returns action=redact, blocked=false, and response_text="Hello [REDACTED]". A missing named rule returns 404.

### Errors, state, and side effects

Evaluate may mask the body in place. Handle external failures according to the configured policy. Debugging rules does not enable them, and blocked/errors must not be ignored.

## Responsibility

Guard configuration, rules and external runners. No model routing or pricing; review external failures, redaction and blocking together.

## Review order

host.go / mount.go → manage.go → guard.go / runner.go → custom_http.go / external.go

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

Host/store, builtins and external runners compose; gateway hooks enforce outcomes.

## Verification

Adjacent tests: custom_test.go, engine_test.go, external_test.go, guard_regression_test.go, guard_test.go, primitives_test.go

From repository root: go test ./internal/gateway/guard -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
