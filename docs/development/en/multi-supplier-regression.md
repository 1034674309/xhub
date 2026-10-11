# Multi-provider model verification

Validate each deployment independently before routing them under one public model name.

~~~bash
go test ./cmd/regression/providerconfig ./e2e/providerconfig -count=1
XHUB_REGRESSION_STRICT=1 go test ./cmd/regression -run '^TestMultiSupplier' -count=1 -v
~~~

| Assertion | Evidence |
| --- | --- |
| Each deployment works | Provider, transport and upstream result |
| Zero weight excluded | Upstream call counts |
| Retry / cooldown / fallback | Actual attempt order and final deployment |
| Successful charge once | Snapshot, logs and five scope totals |
| Cleanup | Schema and dedicated Redis |

Small random samples do not prove exact traffic ratios. Deterministic draws verify weighted intervals; real calls verify protocol, ownership and invoices.

[Paid runs](regression-live.md) · [Template JSON](routing.md)
