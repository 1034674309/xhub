# Guardrails and XGo scripts

[简体中文](../zh-CN/guardrails.md) · [Developer documentation](README.md)

Configure, draft-test, save, bind/enable, and verify real inference. Block prevents generation; redact changes supported upstream text and exchange/cache keys; flag permits and records. Default-enabled rules apply without explicit selection. Storage failure blocks rather than bypassing. ApplyGuardrail uses registered XGo helpers; draft/inference share execution and edits affect subsequent calls. Validate syntax/runtime failures. Supported pre-call Chat/Responses text is not all tool arguments, multimodal content, or official forwarding. External moderation needs configuration/credentials; test unsupported status, timeouts, malformed responses, and configured degradation. Verify monitor totals/details/logs after block/redact/flag.

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
- [Configuration and persistence](configuration.md)
- [Runtime behavior and limits](runtime.md)
- [Deployment and operations](operations.md)
- [Testing and evidence](testing.md)
- [Backend regression](regression.md)
- [Browser acceptance](e2e-regression.md)
- [HTTP API reference](api-reference.md)
- [Qiniu FAL media integration](qiniu-fal.md)

