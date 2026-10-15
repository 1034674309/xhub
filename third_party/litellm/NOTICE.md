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
known deployment ownership before replaying encrypted history. The current DeepSeek
API supports more effort controls and native endpoints than the cited legacy adapter;
XHub preserves provider-supported native values instead of forcing its older on/off mapping.

Additional public-thinking mapping reference (MIT): official LiteLLM
`f08840579ebc072b642573e72f5a11020a3c1bbf`,
`litellm/llms/anthropic/pass_through/adapters/transformation.py` and
`responses_adapters/{transformation,streaming_iterator}.py`.
The Go Messages bridge adapts public reasoning ↔ unsigned thinking and ordered
content blocks. It does not implement that snapshot's encrypted-reasoning envelope
or claim to manufacture Anthropic signatures. Parallel tool fragments are buffered
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
No OpenAI ciphertext is decrypted or fabricated and no signed-thinking envelope
is implemented. Core governance and existing retry rules stay in XHub.
