# 计价与用量

按实际用量和请求开始时适用费率计算；保存当次快照，不用今天的价格重算旧账。

## 计算片段

在导入 fmt 和 internal/catalog 的函数中使用；startedAt 为 time.Time，company-chat 已配置费率。

~~~go
// 示例：计算当次费用；缺少价格时显式返回错误，不当成免费。
charge, priced := catalog.CostAt("company-chat", catalog.Usage{
    PromptTokens: 1500, CompletionTokens: 100,
}, startedAt)
if !priced {
    return fmt.Errorf("model has no applicable price")
}
_ = charge.Total
~~~

| 计量 | 规则 |
| --- | --- |
| 归一化费率 | quantity × usd，不再次除以 unit_size |
| 缓存读取 token | 输入 token 的子集，不重复加输入 |
| 媒体 | 成功结果的实际数量、秒数和规格，不用请求目标时长 |
| 历史日志 | 使用 price_snapshot |
| 队列 | PostgreSQL 事务提交后 Redis Ack |

缺少用量或价格维度时保留待核价，不编造 token、不算免费。当前没有请求预算预占、供应商成本与客户售价独立账本；响应费用头不是持久化收据。异步任务通过成功查询触发去重结算，没有后台补查结算。

## 源码与验证

[计算费率](../../../internal/catalog/cost_at.go) · [计量模块](../../../internal/provider/measurement) · [运行边界](runtime.md)

~~~bash
go test ./internal/catalog ./internal/iam ./internal/live -count=1
~~~
