# 多供应商同模型验证

先分别验证每条部署，再让它们共享公开模型名测试选路。

~~~bash
go test ./cmd/regression/providerconfig ./e2e/providerconfig -count=1
XHUB_REGRESSION_STRICT=1 go test ./cmd/regression -run '^TestMultiSupplier' -count=1 -v
~~~

| 断言 | 证据 |
| --- | --- |
| 独立部署可调用 | provider / transport / 上游返回 |
| 0 权重不到达 | 上游调用计数 |
| 重试 / 冷却 / 回退 | 实际尝试顺序与最终部署 |
| 成功费用只记一次 | 快照、日志、五级累计 |
| 测试后还原 | schema 和临时 Redis 清理 |

少量随机调用不能证明严格分流比例。确定性抽样验证加权区间；真实调用验证协议、归属与实际账单。

[付费入口](regression-live.md) · [模板 JSON](routing.md)
