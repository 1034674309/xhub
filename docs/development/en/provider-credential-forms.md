# Provider credential forms

[简体中文](../zh-CN/provider-credential-forms.md) · [Developer documentation](README.md)

Provider choice determines credential/address/region/project fields. Named credentials are separate from deployments. Hydration copies allowed fields: a nonempty named key replaces the old key; other allowed values fill blanks. Environment references resolve during hydration. Never publish secrets. Validate provider switching, required fields, masked editing, invalid credentials, and discovery failures. Preserve actual user input across language changes.

## Related references

- [Developer documentation](README.md)
- [Architecture and request lifecycle](architecture.md)
- [Implementation map](implementation.md)
- [Models and endpoints](model-endpoints.md)
- [Provider catalog and capabilities](provider-catalog.md)
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
- [HTTP API reference](api-reference.md)
- [Qiniu FAL media integration](qiniu-fal.md)

