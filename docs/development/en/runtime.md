# Runtime behavior and limits

[简体中文](../zh-CN/runtime.md) · [Developer documentation](README.md)

Response cache is a process-local 8192-entry LRU with copied bytes, no TTL or replica invalidation. Only complete nonstreaming successes enter it. Keys include caller/operation/model/body and configuration/routing/session/query digests. Selected deployment/credentials/adapter version are not independent key fields. Hits add zero generation cost. Idempotency-Key replay is process-local/nonstreaming and reauthenticates. Fingerprint conflicts return 409; concurrent identical requests wait. Completed statuses below 500 retain 24h; failure/cancellation releases slots. Restart/another replica can execute again. Task pins last seven days, sessions one hour; local fallback can disappear. Ambiguous task creation returns 502 without blind retry. Official forwarding bypasses ordinary text guardrails/cache. Partial metadata support does not imply all resource operations; unsupported returns 501. Without Redis replicas lack shared rate limits. Prompt logging requires explicit access and retention policies.

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
- [Deployment and operations](operations.md)
- [Testing and evidence](testing.md)
- [Backend regression](regression.md)
- [Browser acceptance](e2e-regression.md)
- [HTTP API reference](api-reference.md)
- [Qiniu FAL media integration](qiniu-fal.md)

