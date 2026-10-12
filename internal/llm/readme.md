# llm

[简体中文](readme_cn.md)

The root provides navigation. Import concrete subpackages directly; no compatibility aliases are retained.

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

This directory organizes credentials, dialogue, and estimate without a root Go API. Select subpackages for credential hydration and dialogue codecs, then send through dataplane. Keep estimates separate from settlement.

| Entry point or resource | Usage and result |
| --- | --- |
| credentials.Hydrate | Connection parameter snapshots |
| dialogue.ParseDialogue / EncodeDialogue | Network-free protocol conversion |
| estimate.CountTokens / TokenCost | Admission/utility estimates |

### Integration demo

Run the following commands from the repository root.

```sh
go test ./internal/llm/... -count=1
```

Verifies child-package contracts. See [dialogue](dialogue/readme.md) for a runnable conversion demo and [credentials](credentials/readme.md) for hydration.

### Errors, state, and side effects

Unified dialogue supports only explicitly representable capabilities; use native entry points for complete vendor fields. Do not import the internal/llm root directory.

## Source and verification

| Directory | Responsibility |
|---|---|
| [dialogue](dialogue/readme.md) | Unified dialogue codecs and strict validation |
| [credentials](credentials/readme.md) | Trusted credential hydration |
| [estimate](estimate/readme.md) | Token and cost estimation |

Vendors depend on registry/measurement and contract. Deployment depends on registry, capability and llm/dialogue. Dataplane executes requests; gateway owns authentication, persistence and composition.

Official openai-go/v3, anthropic-sdk-go and Google genai response types are reused. SSE decoding lives in dataplane/dialogue. Native bypass uses standard HTTP to preserve unknown fields; SDKs do not own retries, routing or settlement.
