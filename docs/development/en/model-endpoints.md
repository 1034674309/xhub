# Models and endpoints

[简体中文](../zh-CN/model-endpoints.md) · [Developer documentation](README.md)

Multiple deployments can publish one public model name. Configure upstream model, credentials, endpoint types, transport, and valid catalog/manual pricing. Omitted endpoint types default to chat. Disabled deployments leave discovery and routing, including pins. Deletion and renaming clean allocation references transactionally. The administrator connection probe uses saved credentials and a minimal English dialogue request. It may charge the supplier outside gateway routing, quota, cooldown, and accounting; media probes are unsupported. Verify gateway inference and logs separately.

## Related references

- [Developer documentation](README.md)
- [Architecture and request lifecycle](architecture.md)
- [Implementation map](implementation.md)
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

