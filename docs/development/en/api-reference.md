# HTTP API reference

[简体中文](../zh-CN/api-reference.md) · [Developer documentation](README.md)

The API tab provides endpoint parameters, authentication, examples, responses, errors, and capability/billing limits. Set the gateway root, normally http://localhost:4000; preserve reverse-proxy prefixes and endpoint version segments. Send Authorization: Bearer YOUR_XHUB_VIRTUAL_KEY. Families cover discovery, Chat/Responses, Anthropic/Gemini, retrieval, images/audio/video, and native forwarding. Deployment support is required; unsupported operations fail explicitly including 501. Management requires authorized sessions, never inference keys. Current Go routes/tests are authoritative; the legacy schema is not verified Go OpenAPI. Generation fails safely rather than using retired Python sources.

## Related references

- [Developer documentation](README.md)
- [Architecture and request lifecycle](architecture.md)
- [Implementation map](implementation.md)
- [Models and endpoints](model-endpoints.md)
- [Provider catalog and capabilities](provider-catalog.md)
- [Provider credential forms](provider-credential-forms.md)
- [Model marketplace and accessible models](model-marketplace.md)
- [Permissions and visibility](permissions.md)
- [Routing templates and execution](routing.md)
- [Routing groups and failover](routing-groups.md)
- [Pricing and usage](pricing.md)
- [Guardrails and XGo scripts](guardrails.md)
- [Configuration and persistence](configuration.md)
- [Runtime behavior and limits](runtime.md)
- [Deployment and operations](operations.md)
- [Testing and evidence](testing.md)
- [Backend regression](regression.md)
- [Browser acceptance](e2e-regression.md)
- [Qiniu FAL media integration](qiniu-fal.md)

