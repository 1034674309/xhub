# Session identity and deployment affinity

Session identity is resolved once for routing, Responses continuation and request logs. Signal order is explicit X-Session-Id or body session_id, previous_response_id, prompt_cache_key, stable prompt fingerprint, then a generated UUID. Conflicting explicit signals return 400. Explicit IDs must be nonempty strings of at most 256 bytes with no control characters or surrounding whitespace. Parent responses are always checked for ownership and consistency.

The session_source values are explicit, parent, cache_key, prompt and generated. Matching explicit and parent IDs retain parent as the source. Completed session resolution adds X-Session-Id to the response; earlier authentication or parsing failures need not include it. Playground creates and persists a UUID, reuses it across refresh and tool calls, and replaces it for a new conversation.

Prompt fingerprints hash initial system/developer instructions and the first meaningful user message, preserving roles and structured content. Appended dialogue and tool definitions do not participate. Original prompts are not stored in the affinity cache. Independent conversations with identical initial content can share an inferred ID; explicit IDs distinguish them.

Affinity is scoped by caller, session, requested model or group and endpoint. The target is a deployment, not a provider brand. Each request validates current authorization, capability, credentials, enabled state, weight and cooldown. A valid preference is tried first; otherwise current model defaults and the selected template determine routing. Retry and fallback policies govern failure handling; successful switching updates the preference without changing session identity.

Templates resolve through key, team and organization without widening model permissions. Affinity uses runtime cache state and does not create a database table. Provider prompt-cache hits remain an upstream property; affinity cannot guarantee a hit or replace actual usage evidence. See [routing](routing.md), [model endpoints](model-endpoints.md) and [runtime](runtime.md).
