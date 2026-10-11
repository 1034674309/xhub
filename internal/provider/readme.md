# provider

The root provides navigation. Import concrete subpackages directly; no compatibility aliases are retained.

| Directory | Responsibility |
|---|---|
| [contract](contract/readme.md) | Shared value contracts |
| [capability](capability/readme.md) | Public endpoints and protocol matching |
| [registry](registry/readme.md) | Built-in registration and path matching |
| [deployment](deployment/readme.md) | Explicit bindings, validation and candidates |
| [measurement](measurement/readme.md) | Response/SSE usage and task billing facts |
| [openai](openai/readme.md) | OpenAI native registration |
| [anthropic](anthropic/readme.md) | Anthropic Messages registration |
| [google](google/readme.md) | Gemini and Vertex registration |
| [qiniu](qiniu/readme.md) | Qiniu task and FAL registration |
| [volcengine](volcengine/readme.md) | Ark task registration |
| [all](all/readme.md) | Startup composition of all vendors |

Vendors depend on registry/measurement and contract. Deployment depends on registry, capability and llm/dialogue. Dataplane executes requests; gateway owns authentication, persistence and composition.
