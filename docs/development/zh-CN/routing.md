# 路由模板

保存完整模板，再绑定密钥、团队或组织。执行按 key → team → organization → platform 选择第一份完整模板，不逐字段合并。

~~~json
{
  "model_routes": [{
    "model": "chat-main", "strategy": "traffic-split",
    "allocations": [
      {"deployment_id": "deployment-a", "weight": 3},
      {"deployment_id": "deployment-b", "weight": 7}
    ]
  }],
  "retry_policy": {
    "max_attempts": 2, "timeout_seconds": 60,
    "failure_threshold": 3, "cooldown_seconds": 60
  },
  "routing_groups": [{
    "group_name": "chat-pool", "models": ["chat-main", "chat-fast"],
    "routing_strategy": "cost-based-routing"
  }],
  "fallbacks": [{"chat-pool": ["chat-backup"]}],
  "context_window_fallbacks": [{"chat-main": ["chat-long"]}],
  "content_policy_fallbacks": []
}
~~~

| 字段 | 规则 |
| --- | --- |
| allocations 省略 | 继承模型默认权重 |
| allocations: [] | 兼容部署权重均为 1 |
| weight: 0 | 排除部署 |
| max_attempts | 每部署总次数，包含首次 |
| timeout_seconds | 单次尝试超时 |
| failure_threshold: 0 | 禁用被动冷却 |
| cooldown_seconds: 0 | 使用 60 秒 |

model_routes 与 retry_policy 必填，可靠性四字段必须完整。拒绝未知字段、失效部署、重复模型、负数或非有限权重、回退环。流已输出不能换部署；异步创建结果不明确不能重放。

策略：simple-shuffle/random 均匀随机；traffic-split 相对权重；least-busy 并发；latency-based-routing 延迟；cost-based-routing 输入单价；usage-based-routing token 用量。

[源码 settings.go](../../../internal/router/settings.go) · [路由组与回退](routing-groups.md)

~~~bash
go test ./internal/router -count=1
~~~
