# Permissions and visibility

[简体中文](../zh-CN/permissions.md) · [Developer documentation](README.md)

Every request uses fresh membership and ownership. Invalid/stale/disabled credentials return 401; forbidden visible actions 403; concealed/missing objects 404; storage failure 500. Platform admins manage all resources, organization admins their organization teams, team admins their team members/projects/service keys, and members their own usage/logs. Account/org/team roles are distinct. Virtual keys never grant management rights. Only owners/platform admins read personal secrets. Password/role/status changes invalidate sessions. SQL lists enforce scope: nil unrestricted differs from empty no-access. Templates cannot expand models. Check template visibility before all binding writes, including bulk updates.

## Related references

- [Developer documentation](README.md)
- [Architecture and request lifecycle](architecture.md)
- [Implementation map](implementation.md)
- [Models and endpoints](model-endpoints.md)
- [Provider catalog and capabilities](provider-catalog.md)
- [Provider credential forms](provider-credential-forms.md)
- [Model marketplace and accessible models](model-marketplace.md)
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

