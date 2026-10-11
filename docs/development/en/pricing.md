# Pricing and usage

Calculate from actual usage and the rate valid at request start. Persist the applied snapshot; do not reprice old calls with today's catalog.

## Example

~~~go
// 示例：供已有 catalog 配置的调用方计算当次费用；无价格时显式处理失败。
charge, priced := catalog.CostAt("company-chat", catalog.Usage{
    PromptTokens: 1500, CompletionTokens: 100,
}, startedAt)
if !priced {
    return fmt.Errorf("model has no applicable price")
}
_ = charge.Total
~~~

## Contract

The fragment runs inside a function importing fmt and internal/catalog; startedAt is a time.Time and company-chat must have configured rates.

| Measure | Rule |
| --- | --- |
| Normalized rate | quantity × usd; do not divide by unit_size again |
| Cache reads | Subset of input tokens, not additional input |
| Media | Actual successful output, not requested duration |
| Historical logs | Read price_snapshot |
| Queue | Commit PostgreSQL before Redis Ack |

There is no budget reservation or separate provider-cost/customer-price ledger. Response cost headers are not a durable commit receipt. Failed/missing billing evidence is not proof of free usage.

## Source and verification

[internal/catalog/cost_at.go](../../../internal/catalog/cost_at.go)

~~~bash
go test ./internal/catalog ./internal/iam ./internal/live -count=1
~~~

[Test prerequisites](testing.md) · [Next: runtime](runtime.md)
