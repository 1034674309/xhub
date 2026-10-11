# Routing templates and execution

[简体中文](../zh-CN/routing.md) · [Developer documentation](README.md)

model_routes and retry_policy are required. traffic-split allocations omitted inherit defaults; [] assigns weight 1; explicit rows override, unlisted deployments use 1, zero excludes. IDs must belong to the model; weights are finite/nonnegative with finite positive sum. Other strategies reject allocations. Uniform random, least-busy, latency, input-unit-cost, and observed-token-usage strategies have distinct selection bases. Resolve one complete key → team → organization → built-in template; no field merge or user/project layer. Sessions inherit only with one team; teamless keys do not inherit owner teams for routing. Preview checks models without quotas, upstream, or bills. max_attempts includes the first attempt per deployment; timeout_seconds is per attempt; failure_threshold zero disables passive cooldown; cooldown_seconds zero uses 60 seconds. Generic/context/content-policy chains reject cycles; empty tables inherit and explicit empty source targets disable. Never restart a sent stream or blindly repeat ambiguous task creation. Delete/rename cleans references atomically.

## Related references

- [Developer documentation](README.md)
- [Architecture and request lifecycle](architecture.md)
- [Implementation map](implementation.md)
- [Models and endpoints](model-endpoints.md)
- [Provider catalog and capabilities](provider-catalog.md)
- [Provider credential forms](provider-credential-forms.md)
- [Model marketplace and accessible models](model-marketplace.md)
- [Permissions and visibility](permissions.md)
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

