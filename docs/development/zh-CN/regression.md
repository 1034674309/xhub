# 后台 regression

## 运行

使用真实网关、独立 PostgreSQL schema 和本地协议上游，不使用生产数据。

~~~bash
make regression
bash scripts/regression.sh -v Pricing
bash scripts/regression.sh -v Fallback
~~~

GitHub Actions 将完整详细输出保存到 `.e2e/release/regression.log`，无论成功
或失败都通过 `release-test-results` 附件上传。步骤成功时只打印简短结果，失败时
仅打印日志最后 16 KiB，并保留回归退出码。本地使用 `-v` 仍打印完整日志。

## 新用例要观察什么

| 场景 | 断言 |
| --- | --- |
| 配置保存 | HTTP 结果 + 重新读取 + 实际执行 |
| 权限 | 正向成功、跨组织/失效身份拒绝、列表范围 |
| 回退 | 上游实际调用顺序、次数、最终模型 |
| 费用 | usage、价格快照、五级累计、重复结算去重 |
| 失败 | 错误响应、无部分写入、无错误成功缓存 |

[用例](../../../cmd/regression) · [隔离 harness](../../../cmd/regression/testsupport/postgres.go)

数据通过 t.Cleanup 清理。数据库不可达不能靠跳过算通过。真实供应商限流/计价另跑[付费验证](regression-live.md)。
