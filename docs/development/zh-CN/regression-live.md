# 真实供应商验证（收费）

这条入口实际发模型请求。只在有供应商调用授权、有效凭据和允许的费用范围时运行。

~~~bash
make regression-live
~~~

scripts/with-live-vendors.py 按 config_provider.yaml 选择供应商，从 key_env 或明确配置的凭据来源读取密钥；密钥不写进 YAML 或日志。

| 前置 | 要求 |
| --- | --- |
| PostgreSQL | 使用独立 schema，结束清理 |
| Redis | runner 独占，不共享测试计数 |
| 模型 | 账户实际可用，价格可核对 |
| 证据 | call ID、provider、deployment、usage、快照、金额 |

缺凭据的 SKIP 不算供应商通过。真实供应商不能稳定制造指定次数的 429/500，精确重试由本地 regression 验证。

[脚本](../../../scripts/with-live-vendors.py) · [多供应商验证](multi-supplier-regression.md) · [免费验证](regression.md)
