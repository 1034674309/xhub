# Configuration and persistence

[简体中文](../zh-CN/configuration.md) · [Developer documentation](README.md)

Startup YAML, Store overrides, IAM resources, templates, and Redis state have different lifecycles. OverlayDB replaces top-level keys; MergePatch recursively merges maps and replaces arrays. Template resolution selects a whole document. Endpoint types default to chat only when omitted; transport defaults to adapted. Only boolean disabled:true pauses. Stable IDs use model_info.id for DB or litellm_params.deployment_id for file entries. Allocations/groups/fallbacks/retry_policy compile and execute, not merely save. Check advanced/legacy fields against readers. Verify exact zero/false/null/empty round trips, atomic failures, disabled discovery, and historical prices.

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
- [Runtime behavior and limits](runtime.md)
- [Deployment and operations](operations.md)
- [Testing and evidence](testing.md)
- [Backend regression](regression.md)
- [Browser acceptance](e2e-regression.md)
- [HTTP API reference](api-reference.md)
- [Qiniu FAL media integration](qiniu-fal.md)

