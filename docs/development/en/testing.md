# Testing and evidence

[简体中文](../zh-CN/testing.md) · [Developer documentation](README.md)

Unit tests verify deterministic modules; backend regression verifies real HTTP/service contracts, persistence, authorization, errors, and cleanup; browser E2E verifies console → gateway → data plane. Page rendering is navigation coverage. Run Go all tests and vet, frontend unit/component/integration/types/lint, root scripts/e2e.sh with isolated ports/report directory, and website unit/build/link/browser checks. Use private schemas and owned data, explicit Redis, cleanup after failures, and queue flush before durable billing assertions. Report exact commands/pass/fail/skip, full versus targeted runs, and external boundaries.

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
- [Backend regression](regression.md)
- [Browser acceptance](e2e-regression.md)
- [HTTP API reference](api-reference.md)
- [Qiniu FAL media integration](qiniu-fal.md)

