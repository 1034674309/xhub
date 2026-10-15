# Agent protocol compatibility and acceptance (Issue #9)

Scope: GPT, Grok and DeepSeek text, function tools, reasoning, Responses and Chat Completions. Images, audio/video and other providers are not certified by this change. A catalog model ID is not evidence of endpoint/parameter support or an available channel for a particular key.

## LiteLLM reference and reuse

References: official LiteLLM `e889d9d6533ac30e776aa3b3780f99fa73e07460` and the existing company fork `f09e043` (v1.95.40-based). Mapping rules are manually adapted into Go, with MIT attribution in [NOTICE](../../../third_party/litellm/NOTICE.md). No Python runtime or enterprise code is imported. This does not give XHub every LiteLLM adapter automatically.

| Concern | LiteLLM reference | XHub behavior |
| --- | --- | --- |
| Native first, bridge second | `responses/main.py`, OpenAI/xAI Responses adapters | Preserve same-protocol requests/results; use the existing shared representation for supported bridges |
| Reasoning and structured output | Completion/Responses transformation | `reasoning.effort` ↔ `reasoning_effort`, `text.format` ↔ `response_format`, public reasoning text ↔ `summary_text` |
| DeepSeek continuation | DeepSeek Chat adapter | Preserve assistant `reasoning_content` across function tool turns, without fabricating missing reasoning |
| Unsupported features | Supported-parameter tables, optional `drop_params` | Preserve native extensions for upstream validation; reject unsupported bridges instead of silently dropping fields |
| Governance | Proxy/Router outside provider transforms | Reuse XHub authentication, quota, guardrails, routing, logging and billing; do not reroute standard APIs through bypass |

Current DeepSeek documentation already describes native Responses and additional effort controls. The referenced legacy adapter still reduces effort to thinking on/off. XHub preserves native values rather than copying that outdated restriction.

## Capability boundaries

| Feature | Native Responses | Native Chat | Responses ↔ Chat |
| --- | --- | --- | --- |
| Text, function tools/results | Supported | Supported | Supported |
| JSON/SSE and observed usage | Preserve native events | Preserve native events and `reasoning_content` deltas | Convert supported text/tool/public reasoning events |
| Reasoning effort | Preserve, model validates values | Preserve, model validates values | Map field names; upstream still validates capability |
| Summaries/encrypted reasoning | Preserve native items | Preserve public reasoning | Public text can be mapped; reject encrypted history and other non-equivalent controls |
| JSON Schema | Preserve | Preserve | Map flattened/nested structures |
| Codex custom/namespace tools and `additional_tools` | Preserve | Not certified here | Reject unsupported items/tools |
| Refusal | Preserve, including native continuation | Preserve | Reject unimplemented conversion explicitly |
| `n > 1`, `background: true` | Reject before dispatch | Reject multiple completions | Unsupported |
| Images/audio/video/other native services | Not certified here | Not certified here | Not certified here |

Clients keep `/v1/responses` or `/v1/chat/completions`, an XHub virtual key and a public model alias. Existing `bypass_openai_responses` / `bypass_openai_chat` deployment names select the **upstream execution transport**; they do not require clients to call `/bypass/...`. Public bypass endpoints still require separate enablement.

The existing managed `store`/`previous_response_id` behavior remains: upstream `store=false`, approximately one hour of caller/model/endpoint-scoped temporary gateway history. Native reasoning and encrypted items now survive that history. This is not permanent OpenAI Response storage and does not implement GET/DELETE/background cancellation lifecycle APIs. HTTP disconnect cancels the upstream request; failed/truncated streams cannot publish successful continuation history. `store=false` does not disable all gateway temporary history.

Full encrypted history requires known session deployment ownership too. It must not switch provider accounts on cooldown, disabled deployment, weight changes or shared-cache failure. Start a new conversation when ownership cannot be established.

Claude Code uses `/v1/messages`, not Responses. Existing bridging is additionally exercised for text/function tools, cache hints, caller metadata, adaptive effort and `clear_thinking_20251015 + keep=all` (no trimming). Cache hints do not guarantee cross-provider cache hits; `metadata.user_id` never replaces gateway identity. Manual thinking budgets, other trimming rules, unknown safeguards/output settings and signed reasoning are outside this bridge and receive explicit errors. Actual Claude Code 2.1.300 retries without unsupported safeguards.

## Live supplier acceptance — 2026-10-15

Authorized Fenno credentials were used against a local new gateway/private PostgreSQL schema. No production containers, models, organization settings or balances were changed. Each model has four checks: Responses JSON/SSE and Chat JSON/SSE, bounded output, successful billing ownership. Passing is a dated observation for that key/request, not certification of all parameters or future versions.

| Company model ID | Passed checks | Observation |
| --- | --- | --- |
| `gpt-5.5` | 0/4 | Upstream 503; direct requests also 503 |
| `gpt-5.6-terra` | 4/4 | Passed |
| `gpt-5.6-sol` | 4/4 | Passed; actual Codex and Claude Code file-tool round trips also passed |
| `codex-auto-review` | 4/4 | Passed; supplier ID, not certification of a separate official model |
| `gpt-6-astra` | 4/4 | Passed |
| `gpt-6-sol` | 4/4 | Passed |
| `gpt-6.1-sol` | 4/4 | Passed |
| `grok-4.5` | 4/4 | Passed |
| `grok-4.6` | 4/4 | Passed |
| `deepseek-v4-flash` | 0/4 | Upstream 503, also fails directly; live acceptance pending |
| `deepseek-v4-pro` | 0/4 | Same |
| `deepseek-v4-1-flash` | 0/4 | Same; preserve supplier spelling rather than guessing an official alias |
| `gpt-image-2` | Not run | Deferred image scope |

32/48 passed; 16 received upstream 503 (existing gateway error contract returns 502). All eight direct controls also returned 503. DeepSeek request/reasoning/stream/tool contracts are covered offline, but cannot be marked live-verified.

Actual Codex `0.162.0-alpha.17.2` and Claude Code `2.1.300` each read a temporary file via a real tool, sent the result through the new gateway to `gpt-5.6-sol`, returned its contents and produced successful billing rows. Pinned release clients Codex `0.162.1` and Claude Code `2.1.300` were separately executed against local protocol fixtures. They are actual binaries, not the existing simulator; simulator coverage remains too.

The fixed offline fixture only issues a test-file read command; protocol acceptance does not depend on the host read-only sandbox implementation. Live supplier tests retain the read-only sandbox. Retries are bounded and timeout cleanup terminates the complete process group, including npm launcher descendants. Windows CLI process-group acceptance is not covered here.

## Regression and reproduction

- Unit: parameter mappings, invalid shapes, public/encrypted reasoning, native/refusal/failed stream results, history and ownership.
- Backend: real HTTP/PostgreSQL, redaction guardrails, authorization/quota, native JSON/SSE, tool continuation, encrypted caller isolation, disabled origin with no failover, cancellation, billing and cleanup.
- Browser: save a deployment, call it in Playground, browser fetch with Agent fields/stream/tool continuation, Chat bridge, verify logs/billing, delete deployment/credentials. Existing JSON/SSE three-turn continuation and Codex simulator log scenarios are also rerun.
- CI: pinned real clients in backend regression; the new browser scenario is a release gate. Client outputs and reports are uploaded on failure too.

```bash
go test -p 2 ./internal/... -count=1 -json
XHUB_TEST_CODEX_BIN=/absolute/path/to/codex \
XHUB_TEST_CLAUDE_BIN=/absolute/path/to/claude \
XHUB_AGENT_REPORT_DIR=/absolute/path/to/reports bash scripts/regression.sh -v
E2E_REDIS=1 E2E_PORTABLE_CONSOLE=1 bash scripts/e2e.sh \
  agent-native-protocol.test.ts scenarios/legacy/responses-continuation scenarios/legacy/codex-agent
node --test scripts/test_release_workflow.cjs
```

PostgreSQL and a local `redis-server` executable are prerequisites. Tests create and clean private schemas. Explicit live mode requires `XHUB_AGENT_LIVE=1`, `XHUB_AGENT_LIVE_KEY`, `XHUB_AGENT_LIVE_BASE` (ending in `/v1`) and optional comma-separated `XHUB_AGENT_LIVE_MODELS` (maximum 12), then `go test ./cmd/regression -run '^Test(LiveTextProtocolMatrix|RealAgentClients)$' -count=1`. Real clients also need their binary paths. This spends supplier credits; CI provides no live credentials. Keep secrets out of code and reports.

References: [LiteLLM Responses](https://docs.litellm.ai/docs/response_api), [OpenAI reasoning](https://developers.openai.com/api/docs/guides/reasoning), [DeepSeek thinking](https://api-docs.deepseek.com/guides/thinking_mode/), [Claude Code gateway](https://code.claude.com/docs/en/llm-gateway). Pinned source references are in the attribution directory.
