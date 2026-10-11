# 配置与存储

按数据所属模块排查，不把部署、身份和路由当成一份配置。

## 找到读写入口

~~~bash
rg 'OverlayDB|MergePatch' internal/config internal/store
rg 'RequestChain|Resolve' internal/router internal/gateway
~~~

| 数据 | 保存位置 | 生效位置 |
| --- | --- | --- |
| 数据库、监听地址 | internal/config | 启动时 |
| 供应商、模型部署 | Store | 模型目录与执行链 |
| 路由模板及绑定 | IAM | 请求时选择完整模板 |
| 用户、成员、密钥 | IAM | 每次鉴权重新读取 |
| RPM、TPM、冷却、费用队列 | Redis | 配置 Redis 时共享 |
| 完整响应缓存 | 进程内 | 重启丢失 |
| 用量与价格快照 | PostgreSQL | 历史账务 |

OverlayDB 替换顶层值，保留 null、0、false 和数组。MergePatch 递归合并 map、替换数组。Resolve 选择一整份模板，不做逐字段继承。身份或模板读取失败不能变成放行。

## 源码与验证

[配置解析](../../../internal/config/config.go) · [部署字段](model-endpoints.md) · [模板 JSON](routing.md)

~~~bash
go test ./internal/router ./internal/store -count=1
~~~
