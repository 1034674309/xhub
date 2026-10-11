# llm

The root provides navigation. Import concrete subpackages directly; no compatibility aliases are retained.

| Directory | Responsibility |
|---|---|
| [dialogue](dialogue/readme.md) | Unified dialogue codecs and strict validation |
| [credentials](credentials/readme.md) | Trusted credential hydration |
| [estimate](estimate/readme.md) | Token and cost estimation |

Vendors depend on registry/measurement and contract. Deployment depends on registry, capability and llm/dialogue. Dataplane executes requests; gateway owns authentication, persistence and composition.

Official openai-go/v3, anthropic-sdk-go and Google genai response types are reused. SSE decoding lives in dataplane/dialogue. Native bypass uses standard HTTP to preserve unknown fields; SDKs do not own retries, routing or settlement.
