# compliance: Compliance-check endpoints for request logs

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Provide a RequireManage Host and mount before catalog fallback. Clients submit log facts; handlers return checks/compliant without running guardrails or persisting records.

| Entry point or resource | Usage and result |
| --- | --- |
| Module(host) | Construct a util.Module and install via Mount/Server.Use |
| POST /compliance/gdpr | HTTP entry point used in this demo |

### Integration demo

Use this integration helper during startup. The host must implement the required interfaces; reg can be a gateway Server. For a standard ServeMux, adapt Handle as in the [health demo](../health/readme.md). The helper compiles but needs a complete host to execute; the HTTP demo below checks the mounted endpoint.

```go
package demo

import (
	"github.com/sunqirui1987/xhub/internal/gateway/compliance"
	"github.com/sunqirui1987/xhub/internal/util"
)

// MountDemo 演示启动时挂载模块；h 提供真实宿主能力，reg 接收路由，返回无。
// 调用方须先初始化依赖；挂载会修改路由表，不执行 HTTP 请求。
func MountDemo(h compliance.Host, reg util.Registrar) { compliance.Module(h).Mount(reg) }
```

Start a configured local gateway first. Set XHUB_DEMO_TOKEN to a session token authorized for this endpoint. See [wire.go](../wire.go) and [routes.go](../routes.go) for complete host wiring; do not substitute nil dependencies.

Run the following commands from the repository root.

```sh
curl -sS -i -X POST "http://localhost:4000/compliance/gdpr" \
  -H "Authorization: Bearer $XHUB_DEMO_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"request_id":"demo-request"}'
```

A result containing compliant, regulation, and checks. Missing facts in this fixture may fail checks; compliant=true is not guaranteed.

### Errors, state, and side effects

request_id is required; missing values return 400. Authentication failures stop processing. Results assess submitted facts and are not legal compliance certification.

## Responsibility

Compliance-check endpoints for request logs. Computes submitted-record checks; no guard execution, persistence or legal assessment.

## Review order

compliance.go: Host / Module → writeCompliance → euAIActChecks / gdprChecks

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

minimal Host for management authorization; util mounts and writes responses.

## Verification

Adjacent tests: compliance_test.go

From repository root: go test ./internal/gateway/compliance -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
