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
