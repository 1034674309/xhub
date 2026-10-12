# execution: Adapted and native request lifecycle orchestration

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Dispatch HTTP requests to Serve or ServeBypass with a complete contract.Adapted/Bypass host supplying authentication, quotas, HTTP, credentials, state, and accounting. The executor coordinates the whole request lifecycle.

| Entry point or resource | Usage and result |
| --- | --- |
| Serve(host,w,r,op) | Unified adapted entry point |
| ServeBypass(host,w,r,hit) | Matched native-action entry point |

### Integration demo

Run an assembled gateway, point demo-model at a local fixture upstream, and set an authorized XHUB_DEMO_TOKEN. See [gateway/wire.go](../../gateway/wire.go) for the real host.

Run the following commands from the repository root.

```sh
curl -sS -i "http://localhost:4000/v1/responses" \
  -H "Authorization: Bearer $XHUB_DEMO_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"model":"demo-model","input":"Hello","stream":false}'
```

A local fixture upstream can return a response object; inspect error on HTTP failure. The model must support responses and the example issues inference.

### Errors, state, and side effects

Handle errors, timeouts, retries, accounting, and stream terminal states together. Never append a different upstream after stream output starts or replay an unknown task-creation result.

## Responsibility

Adapted and native request lifecycle orchestration. No console CRUD or direct PostgreSQL/Redis access; review retries, failed billing and successful completion together.

## Review order

serve.go: Serve → official.go: ServeBypass

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

contract injects identity, quotas, credentials, HTTP and spend; routing, dialogue, transport, stream, usage and diagnostics handle specialized work.

## Verification

Adjacent tests: bypass_logic_test.go, cache_scope_test.go, capability_test.go, endpoint_redesign_test.go, failure_log_test.go, fal_test.go, fallback_boundary_test.go, fallback_test.go, native_bypass_test.go, official_settlement_test.go, official_template_test.go, qiniu_live_test.go, route_diagnostic_test.go, stream_failure_regression_test.go, task_status_test.go, usage_stream_test.go

From repository root: go test ./internal/dataplane/execution -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
