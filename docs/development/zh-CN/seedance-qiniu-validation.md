# Seedance 计量验证

## 本地回归

~~~bash
go test ./internal/provider/... ./internal/catalog ./internal/dataplane/... -count=1
XHUB_REGRESSION_STRICT=1 go test ./cmd/regression -run '^TestSeedance' -count=1 -v
~~~

## 核对内容

| 项目 | 断言 |
| --- | --- |
| 创建与查询 | 同一创建身份、正确模型与任务路径 |
| 等待 / 失败 | 不结算 |
| 成功 | 实际 completion_tokens 与准确价格档 |
| 重复成功查询 | 一条收费事件，五级累计只增加一次 |
| 缺费率 | 保留用量与 unpriced，不标成免费 |

历史一次真实任务只证明该型号当时可调用，不证明所有目录型号或供应商扣款一致。当前无后台补查、预算预占、创建时锁价、补价分录或供应商账单对账。

[任务实现](../../../internal/provider/qiniu/seedance.go) · [付费测试前置](regression-live.md)
