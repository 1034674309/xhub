# Pricing and usage

[简体中文](../zh-CN/pricing.md) · [Developer documentation](README.md)

Use a priced catalog base_model or explicit manual rates. Zero is valid; negative/nonfinite values and invalid rows fail. Units are token/picture/second/query with supported windows and usage dimensions; usd is per base unit. Cached input is an input subset. Response-cache hits add zero generation cost; provider prompt caching still calls upstream. Record request-time ownership/prices; later changes do not rewrite history. request_id deduplicates settlement. Redis Peek → commit → Ack allows delivery retry. Terminal media usage settles once. Response headers do not prove durable commit. Budgets check recorded/hot spend without reserving future cost, so concurrent overspend remains possible.

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
- [Guardrails and XGo scripts](guardrails.md)
- [Configuration and persistence](configuration.md)
- [Runtime behavior and limits](runtime.md)
- [Deployment and operations](operations.md)
- [Testing and evidence](testing.md)
- [Backend regression](regression.md)
- [Browser acceptance](e2e-regression.md)
- [HTTP API reference](api-reference.md)
- [Qiniu FAL media integration](qiniu-fal.md)

