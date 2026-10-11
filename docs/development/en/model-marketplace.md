# Model marketplace and accessible models

[简体中文](../zh-CN/model-marketplace.md) · [Developer documentation](README.md)

Catalog entries are distinct from deployments and caller-accessible models. They do not prove credentials, permissions, or live availability. My Models uses enabled deployments and scoped access. Unauthenticated GET /models exposes safe public metadata; explicit credentials enforce scope and invalid keys return 401. GET /v1/models retains authentication. Secrets and deployment addresses are omitted.

## Related references

- [Developer documentation](README.md)
- [Architecture and request lifecycle](architecture.md)
- [Implementation map](implementation.md)
- [Models and endpoints](model-endpoints.md)
- [Provider catalog and capabilities](provider-catalog.md)
- [Provider credential forms](provider-credential-forms.md)
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

