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

The follow-up references official `35970c3eacb9f4f8746af215586f23f3a2a17941`: namespace/custom round trips, optional reasoning hints, required Responses usage details, and late Messages input-token reporting. XHub also projects inline `additional_tools` and public reasoning context: `current_turn` keeps public reasoning after the final user message, while `auto/all_turns` keep available history. It neither decrypts nor fabricates vendor ciphertext. Client trace metadata never replaces gateway identity.

## Capability boundaries

| Feature | Native Responses | Native Chat | Responses ↔ Chat |
| --- | --- | --- | --- |
| Text, function tools/results | Supported | Supported | Supported |
| JSON/SSE and observed usage | Preserve native events | Preserve native events and `reasoning_content` deltas | Convert supported text/tool/public reasoning events |
| Reasoning effort | Preserve, model validates values | Preserve, model validates values | Map field names; upstream still validates capability |
| Summaries/encrypted reasoning | Preserve native items | Preserve public reasoning | Public text maps; Responses output survives in the Chat `reasoning_items` extension; projecting raw Responses ciphertext history into ordinary Chat still fails |
| JSON Schema | Preserve | Preserve | Map flattened/nested structures |
| Codex custom/namespace tools and `additional_tools` | Preserve | Function representation | Flatten/restore single-level namespaces; wrap custom strings as function JSON; project inline declarations; grammar becomes a model hint |
| Refusal | Preserve, including native continuation | Preserve | Reject unimplemented conversion explicitly |
| `n > 1`, `background: true` | Reject before dispatch | Reject multiple completions | Unsupported |
| Images/audio/video/other native services | Not certified here | Not certified here | Not certified here |

Clients keep `/v1/responses` or `/v1/chat/completions`, an XHub virtual key and a public model alias. Existing `bypass_openai_responses` / `bypass_openai_chat` deployment names select the **upstream execution transport**; they do not require clients to call `/bypass/...`. Public bypass endpoints still require separate enablement.

The existing managed `store`/`previous_response_id` behavior remains: upstream `store=false`, approximately one hour of caller/model/endpoint-scoped temporary gateway history. Native reasoning and encrypted items now survive that history. This is not permanent OpenAI Response storage and does not implement GET/DELETE/background cancellation lifecycle APIs. HTTP disconnect cancels the upstream request; failed/truncated streams cannot publish successful continuation history. `store=false` does not disable all gateway temporary history.

Full encrypted history requires known session deployment ownership too. It must not switch provider accounts on cooldown, disabled deployment, weight changes or shared-cache failure. Start a new conversation when ownership cannot be established.

Fixed continuations also verify a fingerprint of the resolved connection used at creation. Keeping a deployment ID while changing its address, upstream model, transport, credential name or a named credential's key makes old continuations return 409. Plain requests updating routing preferences cannot rebind old ciphertext. Cached ownership predating this fingerprint requires a new conversation; the cache stores a hash, never the plaintext key.

Public reasoning now maps to unsigned Messages `thinking` blocks following LiteLLM. Tool turns restore Chat `reasoning_content` or separate Responses reasoning-summary items. Streaming thinking/text blocks close in order; interleaved tool arguments are emitted serially only after completion validation. Codex history with `encrypted_content: null` is a nullable placeholder. Following LiteLLM `dba86bec28cb483c9937470c5d950f5e03699d3b`, Responses → Messages now wraps summary plus ciphertext in `thinking.signature`, or ciphertext alone in `redacted_thinking.data`, using `xhub_responses_reasoning:` rather than an Anthropic signature. Tool continuation restores the original Responses ciphertext without inventing an item ID and requires the same caller, public model, session and supplier connection. SSE emits the signature before closing the completed reasoning block and validates public content at the terminal response. Re-encryption of the same reasoning may change ciphertext bytes; the emitted envelope remains intact. Replaying a Messages envelope into native Chat/Anthropic or another account remains rejected; genuine supplier signatures still require native transport.

Claude Code uses `/v1/messages`, not Responses. Existing bridging is additionally exercised for text/function tools, cache hints, caller metadata, adaptive effort and `clear_thinking_20251015 + keep=all` (no trimming). Cache hints do not guarantee cross-provider cache hits; `metadata.user_id` never replaces gateway identity. Manual thinking budgets, other trimming rules, unknown safeguards/output settings and signed reasoning are outside this bridge and receive explicit errors. Actual Claude Code 2.1.300 retries without unsupported safeguards.

## Live supplier acceptance — 2026-10-15

Authorized Fenno (GPT/Grok) and Qiniu (DeepSeek) connections were tested against an independent local gateway/database. Production containers, models, organizations and balances were not changed. Each model has four checks: Responses JSON/SSE and Chat JSON/SSE. Passing is a dated observation for that key/request, not certification of every parameter or future version.

| Supplier | Upstream model ID | Four checks |
| --- | --- | --- |
| Fenno | `gpt-5.5` | 0/4; upstream 503, also 503 directly on both endpoints |
| Fenno | `gpt-5.6-terra`, `gpt-5.6-sol`, `codex-auto-review`, `gpt-6-astra`, `gpt-6-sol`, `gpt-6.1-sol` | 4/4 each |
| Fenno | `grok-4.5`, `grok-4.6` | 4/4 each |
| Qiniu | `deepseek/deepseek-v4.1-flash` | 4/4 |
| Qiniu | `deepseek/deepseek-v4-pro-0813` | 4/4 |
| Qiniu | `deepseek/deepseek-v4-flash-20260731` | 4/4 |

The API matrix passed **44/48; four GPT-5.5 availability checks failed**. The existing gateway error contract maps upstream 503 to 502. Raw responses are retained, and the probe treats normal `error: null` as no error. Supplier IDs do not certify separate official models. Qiniu returned 404 on native `/responses`, so DeepSeek uses Chat upstream with XHub Responses/Messages bridging. Earlier Fenno DeepSeek 503 results are not attributed to Qiniu.

Pinned Codex **0.162.1** and Claude Code **2.1.300** actually read a temporary file, returned its tool result, and answered with a marker from that file. The marker was absent from the prompt.

| Local deployment | Codex | Claude Code |
| --- | --- | --- |
| `gpt-5.6-sol` (native Responses) | Passed | Passed |
| `gpt-5.6-sol-chat` (Chat bridge) | Passed after cooldown | Passed |
| `grok-4.6` (native Responses) | Passed | Passed |
| `grok-4.6-chat` (Chat bridge) | Passed | Passed |
| `qiniu-deepseek-v4.1-flash` (Chat bridge) | Passed | Passed |
| `qiniu-deepseek-v4-pro-0813` (Chat bridge) | Passed | Passed |
| `qiniu-deepseek-v4-flash-20260731` (Chat bridge) | Passed | Passed |

The initial client matrix passed 13/14. A preceding upstream stream error put the GPT Chat deployment in cooldown; an unchanged retry after cooldown passed, completing 14/14 tool round trips. The first failure is retained, and retry success is not an availability guarantee. Earlier GPT/Grok Responses/Chat/Messages JSON/SSE checks passed 24/24; the three Qiniu models passed 18/18 across those endpoints (including serial retries after rate limiting). Manual test rates validate accounting only; they are not supplier prices.

File-tool acceptance disables hosted web search and external connectors. The generic Chat bridge does not silently discard these tools; reasoning effort, summary and include requests remain enabled. Hosted search, images, signed/encrypted cross-provider replay and long-session compaction are not certified. The console `/gw` proxy timeout is ten minutes, with independent gateway/client deadlines; a real 31-second non-streaming browser regression covers the previous disconnect.

Offline fixtures only issue test-file reads; protocol acceptance does not depend on host read-only sandbox support. Live tests retain the read-only sandbox. Retries are bounded, and timeout cleanup terminates the complete process group. Windows process-group acceptance is not covered.

Executed checks: internal unit tests 687 passed/4 skipped; backend regression 225 passed/12 skipped (including four offline tool round trips with two actual clients and two upstream protocols); browser E2E 4/4; upstream fixture unit tests 14/14; Linux release-workflow tests 11/11. After the alias-collision fix, focused unit tests passed 50/50 and the backend bridge regression passed again. macOS Bash 3.2 exposes one existing empty-array release-script failure; the Linux/CI Bash environment passes all checks.

## Regression and reproduction

- Unit: parameter mappings, invalid shapes, public/encrypted reasoning, native/refusal/failed stream results, history and ownership.
- Backend: real HTTP/PostgreSQL, redaction guardrails, authorization/quota, native JSON/SSE, tool continuation, encrypted caller isolation, disabled origin with no failover, cancellation, billing and cleanup.
- Browser: a dedicated Messages case covers both OpenAI upstreams, public thinking, JSON/SSE tool continuation and billing logs; save a deployment, call it in Playground, browser fetch with Agent fields/stream/tool continuation, Chat bridge, verify logs/billing, delete deployment/credentials. Existing JSON/SSE three-turn continuation and Codex simulator log scenarios are also rerun.
- CI: pinned real clients in backend regression; the new browser scenario is a release gate. Client outputs and reports are uploaded on failure too.

```bash
go test -p 2 ./internal/... -count=1 -json
XHUB_TEST_CODEX_BIN=/absolute/path/to/codex \
XHUB_TEST_CLAUDE_BIN=/absolute/path/to/claude \
XHUB_AGENT_REPORT_DIR=/absolute/path/to/reports bash scripts/regression.sh -v
E2E_REDIS=1 E2E_PORTABLE_CONSOLE=1 bash scripts/e2e.sh \
  agent-native-protocol.test.ts messages-reasoning.test.ts agent-chat-bridge.test.ts agent-proxy-timeout.test.ts scenarios/legacy/responses-continuation scenarios/legacy/codex-agent
node --test scripts/test_release_workflow.cjs
```

PostgreSQL and a local `redis-server` executable are prerequisites. Tests create and clean private schemas. Explicit live mode requires `XHUB_AGENT_LIVE=1`, `XHUB_AGENT_LIVE_KEY`, `XHUB_AGENT_LIVE_BASE` (ending in `/v1`) and optional comma-separated `XHUB_AGENT_LIVE_MODELS` (maximum 12), then `go test ./cmd/regression -run '^Test(LiveTextProtocolMatrix|RealAgentClients)$' -count=1`. Real clients also need their binary paths. This spends supplier credits; CI provides no live credentials. Keep secrets out of code and reports.

References: [LiteLLM Responses](https://docs.litellm.ai/docs/response_api), [OpenAI reasoning](https://developers.openai.com/api/docs/guides/reasoning), [DeepSeek thinking](https://api-docs.deepseek.com/guides/thinking_mode/), [Claude Code gateway](https://code.claude.com/docs/en/llm-gateway). Pinned source references are in the attribution directory.

Claude Code sessions: Messages prefers gateway `X-Session-Id` / `session_id`, then
`X-Claude-Code-Session-Id`, or the session component of JSON-string / legacy
`user_…_account_…_session_…` values in `metadata.user_id`. Both explicit sources
retain the `explicit` log classification. This follows the same official LiteLLM
snapshot's session extraction and survives first-user/cache-hint changes during
tool turns. Plain user, device and account IDs never identify a conversation.
Native headers require 1–256 ASCII letters, digits, hyphens or underscores;
conflicting headers fail, while unrecognized optional metadata uses the existing
inference fallback. Caller/model/endpoint/connection checks still apply. Existing
ciphertext conversations without native session ownership need a fresh session.


## Responses reasoning through Chat

Following LiteLLM `dba86bec28cb483c9937470c5d950f5e03699d3b`, `completion_extras/litellm_responses_transformation/transformation.py`, Chat clients using a Responses deployment receive normal content, tool calls and usage. Public summaries map to `reasoning_content`; the original reasoning `id`, `summary` and `encrypted_content` travel in the assistant message's `reasoning_items` extension. Ciphertext is neither decrypted nor exposed as visible text.

Ordinary clients may ignore this extension and continue with visible messages and tool results, without persisted reasoning or an equivalent-quality guarantee. To preserve reasoning, replay the assistant's `reasoning_items` unchanged, echo the response's `X-Session-Id`, and append the tool result. Only assistant reasoning arrays are accepted. Replay requires the same trusted caller, public model, endpoint, session and supplier connection, targeting Responses; unknown ownership or changed connections fail. The extension is never forwarded to native Chat, Anthropic or another provider.

SSE publishes the extension only with the finish chunk after validating the upstream terminal response, followed by usage and `[DONE]`; terminal re-encryption keeps the terminal value. Ownership is committed before that chunk. Failed, malformed or truncated streams do not fabricate completion. Explicit `response.incomplete` retains `finish_reason: length` even when tools are present. This is a LiteLLM-style extension rather than an OpenAI Chat standard field; ignoring clients do not gain hidden reasoning memory.
