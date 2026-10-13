# Seedance measurement verification

## Local regression

~~~bash
go test ./internal/provider/... ./internal/catalog ./internal/dataplane/... -count=1
XHUB_REGRESSION_STRICT=1 go test ./cmd/regression -run '^TestSeedance' -count=1 -v
~~~

## Assertions

| Area | Expected result |
| --- | --- |
| Create / query | Creation identity, correct model and task path |
| Waiting / failure | No settlement |
| Success | Actual completion_tokens and exact pricing variant |
| Repeated success queries | One event; five scope totals increment once |
| Missing rate | Preserve usage and unpriced; not free |

Validate live supplier tasks for each model independently; local task tests do not establish supplier availability or invoice amounts. No background polling, budget reservation, create-time price locking, repricing entries or provider invoice reconciliation exists.

[Task source](../../../internal/provider/qiniu/seedance.go) · [Paid prerequisites](regression-live.md)
