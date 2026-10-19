# LiteLLM protocol adaptation references

The Go adapters in `internal/llm/dialogue/native.go`, `request.go`, `response.go` and
`internal/dataplane/dialogue/dialogue_stream.go` adapt the following MIT-licensed
LiteLLM mapping ideas and representations rather than importing a Python runtime:

- Native Responses first, Chat Completion bridge when no native Responses adapter exists:
  `litellm/responses/main.py`, `litellm/llms/openai/responses/transformation.py`.
- `reasoning.effort` ↔ `reasoning_effort`, flattened Responses JSON Schema ↔ nested
  Chat `response_format`, and reasoning text ↔ Responses `summary_text`:
  `litellm/responses/litellm_completion_transformation/transformation.py`.
- Preserve DeepSeek assistant `reasoning_content` across tool turns:
  `litellm/llms/deepseek/chat/transformation.py`.

Reference snapshots:
- Official BerriAI/litellm: `e889d9d6533ac30e776aa3b3780f99fa73e07460`.
- Existing company fork adam-qin/litellm: `f09e043` (v1.95.40-based).

These are manually adapted Go implementations and bounded compatibility behavior,
not a claim that XHub inherits every LiteLLM provider or model feature. Existing
XHub authorization, routing, guardrails, stream decoder and billing remain in use.
No code from LiteLLM `enterprise/` is included. See [LICENSE](LICENSE).

Changes from the reference include explicit unsupported-bridge errors rather than
silently dropping parameters, no fabricated blank DeepSeek reasoning, and requiring
known deployment ownership before replaying opaque history on native endpoints.
The unified Responses entry instead uses the portable-history policy documented below. The current DeepSeek
API supports more effort controls and native endpoints than the cited legacy adapter;
XHub preserves provider-supported native values instead of forcing its older on/off mapping.

Additional public-thinking mapping reference (MIT): official LiteLLM
`f08840579ebc072b642573e72f5a11020a3c1bbf`,
`litellm/llms/anthropic/pass_through/adapters/transformation.py` and
`responses_adapters/{transformation,streaming_iterator}.py`.
The Go Messages bridge adapts public reasoning ↔ unsigned thinking and ordered
content blocks. The encrypted-reasoning envelope is covered by the newer reference
below; XHub does not manufacture Anthropic signatures. Parallel tool fragments are buffered
until JSON validation and emitted serially using XHub's existing stream state.

Additional Codex bridge and usage reference (MIT): official LiteLLM
`35970c3eacb9f4f8746af215586f23f3a2a17941` (local upstream reference updated
2026-10-15; company branch `xhub-qiniu` remains at `f09e043`). Relevant files:
`responses/litellm_completion_transformation/{transformation,custom_tools}.py`
and `llms/anthropic/pass_through/adapters/{transformation,streaming_iterator}.py`.
The Go adaptation flattens/restores namespace functions, wraps/unwraps custom
string inputs, separates optional Responses reasoning hints, fills required
Responses usage detail fields and returns late Chat usage in Messages events.
XHub additionally projects Codex inline `additional_tools` declarations and
scopes public reasoning for `current_turn`; it does not claim these extensions
are direct copies of LiteLLM. Long tool names use a stable bounded alias and
tool-name collisions fail explicitly. Custom grammar is a best-effort model
description on Chat, not a native grammar guarantee. Hosted web search remains
unsupported on the generic Chat bridge and is disabled in file-tool acceptance.
No OpenAI ciphertext is decrypted or fabricated. Core governance and existing retry rules stay in XHub.


Additional Responses → Messages encrypted-reasoning reference (MIT): official
LiteLLM `dba86bec28cb483c9937470c5d950f5e03699d3b` (2026-10-16),
`llms/anthropic/pass_through/responses_adapters/{transformation,streaming_iterator}.py`
and `litellm_core_utils/prompt_templates/common_utils.py`.
XHub adapts the opaque round trip: summary plus ciphertext becomes
`thinking.signature`; ciphertext without a summary becomes `redacted_thinking.data`.
Its own `xhub_responses_reasoning:` namespace distinguishes these values from
Anthropic signatures. Replay restores a Responses reasoning item without inventing
an item ID. Completed SSE items carry the signature before their block closes;
the terminal response validates public content and fills missing items. Providers may
re-encrypt the same reasoning at the terminal snapshot; the already emitted opaque
envelope remains intact rather than requiring ciphertext equality. XHub additionally requires
known caller/model/session/connection ownership before replay and rejects carrying
this ciphertext into Chat, native Anthropic or another supplier account. This is
protocol adaptation, not encryption, Anthropic signature generation or general
cross-vendor reasoning compatibility.


Claude Code session extraction additionally adapts the same official snapshot's
`proxy/litellm_pre_call_utils.py` (`_get_anthropic_session_id_from_metadata` and
native session-header handling) and `llms/anthropic/pass_through/utils.py`
(`prompt_cache_key_from_user_id`). XHub reads only the known Claude header or the
session component of JSON/legacy `metadata.user_id`; plain user/device/account IDs
never identify a conversation. Explicit gateway session IDs keep priority. Unlike
the reference's general header matching, XHub limits this fallback to Messages,
bounds and validates IDs, and keeps caller/model/endpoint/connection ownership.


The Responses → Chat reasoning extension additionally adapts the same official
`dba86bec28cb483c9937470c5d950f5e03699d3b` snapshot's
`completion_extras/litellm_responses_transformation/transformation.py`:
`_build_reasoning_item`, `_reasoning_items_from_output_items`,
`_convert_response_output_to_choices`, `_reasoning_input_items`, and
`OpenAiResponsesToChatCompletionStreamIterator`. Public summaries remain in
`reasoning_content`; original reasoning IDs and ciphertext survive in optional
`reasoning_items`, including the terminal SSE delta. XHub uses its existing
Messages field validation, native Turn encoding and caller/model/endpoint/session/
connection ownership. It neither decrypts ciphertext nor forwards this extension
to unrelated native Chat/Anthropic deployments. Clients dropping the extension
retain ordinary Chat behavior without a persisted-reasoning guarantee.

Optional web search downgrade reference (MIT): LiteLLM `35970c3`,
`responses/litellm_completion_transformation/transformation.py`,
`_responses_tool_to_chat_form` and `_should_drop_derived_web_search_options`.
XHub reuses the provider-capability decision pattern, not the Python runtime.
Unlike the reference's implicit drop, XHub defaults to rejection, requires an explicit
per-deployment `drop_optional` policy, rejects required search and exposes the downgrade
in response headers and persisted diagnostics. Native Responses search remains unchanged
unless the deployment explicitly declares it unsupported. External search execution is
not implemented by this change.

Custom-input and client-tool-discovery follow-up references (MIT): the same
`35970c3` snapshot's `custom_tools.py` (`convert_custom_tool_to_function_tool`,
`unwrap_custom_tool_arguments`), `transformation.py` (`_responses_tool_to_chat_form`),
and `streaming_iterator.py` (tool-search item lifecycle). XHub keeps the reference's
`content` string schema and adds a bounded `input` string alias observed from Qiniu
DeepSeek; unlike LiteLLM's raw-string fallback it still rejects malformed JSON,
non-string wrappers and ambiguous extra fields, without repairing executable text.
LiteLLM drops client-executed tool search on its generic Chat bridge. XHub instead
implements an explicit client-only function projection, object-valued search calls,
search-result history and client-provided tool definitions according to the
[OpenAI client tool-search contract](https://developers.openai.com/api/docs/guides/tools-tool-search).
It does not execute search in the gateway or pretend server-executed search is supported.
Identical loaded declarations deduplicate; conflicting aliases remain errors. Native
requests preserve their tool definitions and discovery history unchanged.


Portable Responses history reference (MIT): official LiteLLM
`35970c3eacb9f4f8746af215586f23f3a2a17941`,
`litellm/router_utils/pre_call_checks/encrypted_content_affinity_check.py`
(`strip_reasoning_the_targets_cannot_decrypt`) and `litellm/responses/utils.py`
(`strip_encrypted_reasoning_from_input`, `_without_encrypted_reasoning`).
The reference removes ciphertext and its origin ID when the target cannot decrypt it,
retaining readable summary/content and tool history. XHub adapts this idea in Go, with
an additional portable-history contract for its unified `/v1/responses`: client-supplied
reasoning and every outgoing JSON/SSE reasoning item omit ciphertext, origin IDs and
provider-specific output status. Native bypass does not change. Original validated
upstream output is retained only in the existing caller-scoped temporary history for
explicit `previous_response_id` continuation. XHub does not decode, wrap or generate
ciphertext, does not edit client-side stored history, and cannot recover plaintext
from opaque compaction items. This deliberately trades transferable hidden reasoning
state for model/provider switching without poisoning the client's history.


Vision input reference (MIT): the same `35970c3eacb9f4f8746af215586f23f3a2a17941`
snapshot's `litellm/responses/litellm_completion_transformation/transformation.py`
(`_transform_input_image_item_to_image_item`,
`_transform_responses_api_content_to_chat_completion_content`) and
`litellm/litellm_core_utils/prompt_templates/factory.py`
(`create_anthropic_image_param`, `convert_generic_image_chunk_to_openai_image_obj`).
XHub adapts the ordered text/image representation and the Responses `input_image`
↔ Chat `image_url` ↔ Messages `image.source` field mappings in Go. Base64 bytes,
MIME, image order and OpenAI detail values are retained. It does not reuse the Python
runtime, fetch remote URLs, transform image pixels, or infer a model's visual capability.
Unlike reference paths that download remote images, XHub passes HTTP(S) URL references
through to the provider. Unavailable file-reference bridges, image tool results and
unrepresentable explicit detail settings remain explicit errors.
