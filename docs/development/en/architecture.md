# Architecture and request lifecycle

[简体中文](../zh-CN/architecture.md) · [Developer documentation](README.md)

The console calls gateway APIs. Authentication and fresh IAM scope precede limits and routing. The router compiles candidates; the data plane executes guardrails, transport, streaming, fallback, usage, and logs. Store persists configuration; IAM owns identity and accounting. PostgreSQL supplies durability; Redis coordinates configured hot state.

## Related references

- [Developer documentation](README.md)
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
- [HTTP API reference](api-reference.md)
- [Qiniu FAL media integration](qiniu-fal.md)

