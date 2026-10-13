# Data plane architecture

[简体中文](readme_cn.md)

The root contains documentation only. Responsibilities for inference/native-task requests are organized below.

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

This directory has no root entry point. Gateway implements contract and calls execution, which coordinates routing, dialogue/transport, stream, diagnostics, usage, and runtime. Utility demos do not replace the full request chain.

| Entry point or resource | Usage and result |
| --- | --- |
| contract.Adapted / Bypass | Host dependency interfaces |
| execution.Serve / ServeBypass | Complete request-lifecycle entry points |
| runtime.Flush | Acknowledge usage queues after database commit |

### Integration demo

Run the following commands from the repository root.

```sh
go test ./internal/dataplane/... -count=1
```

Runs data-plane package tests. See [execution](execution/readme.md) for an HTTP request demo and [dialogue](dialogue/readme.md) for offline continuation-body construction.

### Errors, state, and side effects

Authenticate and enforce quotas before sending. Cache hits and upstream success are different facts. Adapted dialogue streams save continuation history only after validating an acceptable terminal state. Responses saves history before writing the terminal event; a subsequent client write failure does not roll back that history. See stream for the terminal-validation boundary of raw forwarding. Vendor availability needs separate environment validation.

## Source and verification

| Package | Responsibility |
|---|---|
| [contract](contract/readme.md) | Execution capability interfaces and call facts |
| [diagnostics](diagnostics/readme.md) | Raw upstream facts and safe diagnostic logs |
| [dialogue](dialogue/readme.md) | Unified dialogue conversion, continuation and adapted SSE |
| [execution](execution/readme.md) | Adapted and native request lifecycle orchestration |
| [routing](routing/readme.md) | Candidate filtering, model fallbacks and routing diagnostics |
| [runtime](runtime/readme.md) | Runtime snapshots, cooldown, latency and spend flushing |
| [stream](stream/readme.md) | Raw stream forwarding and read-only usage observation |
| [transport](transport/readme.md) | Native HTTP, payloads, paths, authentication and responses |
| [usage](usage/readme.md) | Usage normalization, missing-count estimation and first-token timing |

## Flow and review order

1. gateway/ingress.go, limits.go and bypass.go: identity and quotas.
2. contract: consumer capabilities; execution/serve.go or official.go: lifecycle.
3. routing: candidates/fallbacks; dialogue: adaptation; transport: native forwarding; stream: unchanged streams.
4. usage/diagnostics: facts; gateway/billing and requestlog: settlement/log preparation; spend.go: persistence.
5. runtime: independent snapshots, cooldown, latency and asynchronous flushing.

execution does not depend on gateway.Server; gateway composes dependencies. contract currently reuses prefs.RouteSettings values without mounting prefs handlers.

## Official SDKs and forwarding boundaries

- [OpenAI Go](https://github.com/openai/openai-go): github.com/openai/openai-go/v3 v3.74.0 for Chat Completions/Responses types and SSE decoding; supports project Go 1.25. Check toolchain requirements before upgrading.
- [Anthropic Go](https://github.com/anthropics/anthropic-sdk-go): v1.80.0 for Messages response types.
- [Google Gen AI Go](https://github.com/googleapis/go-genai): google.golang.org/genai v1.73.0 for Gemini/Vertex response types.

internal/llm/dialogue uses official response types; dialogue/PipeDialogue uses official SSE framing. SDKs can parse invalid field types leniently, so adaptation still validates text, tools and terminal events. Invalid types and unsupported capabilities fail explicitly; raw usage preserves absence versus explicit zero.

SDKs are calling clients, not complete gateway bypass, conversion, quotas, routing, logging or settlement libraries. transport keeps a thin Go net/http forwarding layer to preserve vendor extensions, raw statuses/errors, multipart and SSE bytes. Cross-protocol conversion remains project-owned. SDK request clients are not instantiated, avoiding hidden SDK retries on top of gateway retries; native stream observation and log reconstruction do not re-encode client responses through SDK types.

## Test layers

Units stay beside source; real HTTP/persistence contracts live in cmd/regression; browser-to-gateway-to-local-upstream flows live in frontend/e2e. Do not create test-only integration packages.

From repository root:

    go test ./internal/dataplane/... ./internal/gateway/... ./internal/llm/... ./internal/provider/... -count=1
    bash scripts/regression.sh 'TestComplianceModuleContract|TestUpstreamDiagnosticsContract|TestResponsesFailedStreamCannotContinue' -v
    bash scripts/e2e.sh scenarios/legacy/playground-transport scenarios/legacy/responses-continuation scenarios/legacy/model-fallback

Tests use real local HTTP/PostgreSQL and isolated fixtures, cleaning private schemas without overwriting user data. Paid vendors/external guards need declared credentials; offline tests cover configuration, support, refusal and failure paths.
