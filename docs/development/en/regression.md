# Backend regression

[简体中文](../zh-CN/regression.md) · [Developer documentation](README.md)

Exercise real gateway routes, PostgreSQL, and local protocol upstreams in private schemas. Verify fields/status, core results, permissions, errors, persistence, cleanup. Direct module calls do not establish HTTP wiring. Run XHUB_REGRESSION_STRICT=1 go test ./cmd/regression -count=1 -timeout=20m -json; explicitly configure Redis for shared state. Flush private billing queue before durable-log assertions.

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
- [Browser acceptance](e2e-regression.md)
- [HTTP API reference](api-reference.md)
- [Qiniu FAL media integration](qiniu-fal.md)

