# 路由模板设计

保存完整模板，再绑定密钥、团队或组织。执行按 key → team → organization 选择第一份完整模板，不逐字段合并。没有绑定时使用平台执行参数和模型管理默认权重、回退。

模板决定本次请求的策略、分配、重试、冷却、路由组和三类回退；[模型与端点](model-endpoints.md)提供部署及执行能力，[会话粘性](session-affinity-design.md)在当前合法候选中优先复用目标。自动故障切换由这套执行规则完成。

## 模板字段

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
| 模型规则省略 allocations | traffic-split 下继承模型默认权重 |
| 模型规则 allocations: [] | 当前合法部署等权，权重均为 1 |
| allocations 未列出的部署 | 权重为 1；分配列表不是候选白名单 |
| weight: 0 | traffic-split 下排除该部署，包括已有粘性目标 |
| max_attempts | 每部署总次数，包含首次 |
| timeout_seconds | 单次尝试超时 |
| failure_threshold: 0 | 禁用被动冷却 |
| cooldown_seconds: 0 | 使用 60 秒 |

`model_routes` 与 `retry_policy` 必填，重试四字段必须完整。`allocations` 仅适用于 traffic-split，非空分配至少包含一个正权重。保存和执行均校验字段、部署引用、重复项、权重与跨类别回退环。

同一模板中，精确模型规则优先于所在组策略；成员请求继承组策略时仍只选择自身部署。显式模型分配使用模型规则的 `allocations`，组分配使用 `routing_strategy_args.allocations`。普通模型无显式分配时继承模型默认权重；组分配缺省等权，不叠加成员默认权重。

模板顶层不接受 `routing_strategy`；策略只属于精确模型规则或路由组。路由组只随当前完整模板保存和生效，不读取独立全局组；省略 `routing_groups` 或使用 `[]` 均表示该模板没有路由组。

三类回退按源模型/组和错误类别覆盖模型默认，其他源和类别继续继承。省略回退表或使用空表 `[]` 均表示继承；`[{"chat-main": []}]` 禁用该源、该类别的回退。`routing_groups: []` 则表示本模板不使用路由组。完整规则见[路由组与回退](routing-groups.md)。

## 选择与故障切换

动态会话偏好优先，目的是保持同一供应商及上游部署，提高 prompt 缓存命中机会。合法偏好优先于权重、价格、负载与延迟。新会话、偏好过期或失效时，按“模型默认配置 → 当前模板的模型规则与路由组”得到本轮路由；精确模型规则优先于组策略。

每次请求取得当前配置快照：选模板 → 展开模型/组 → 过滤端点能力、启用状态、权重与冷却 → 验证粘性目标 → 按策略尝试。发送前检查实际目标的权限、预算和凭据。配置变化在后续请求重新生效，粘性不固定旧模板。

可重放的通用失败先在同一部署按 `retry_policy.max_attempts` 完成重试，再尝试池内其他部署，池耗尽后进入 `fallbacks`；已识别的上下文超限或上游内容策略错误可以直接进入对应专用回退。回退目标按自身当前策略和权重调度。`max_attempts: 3` 表示同一部署首次调用加两次重试；没有回退链或禁用跨模型回退也不取消安全的同部署重试。当前请求的重试预算不因自身失败触发冷却而被截断，后续请求跳过冷却部署。

请求 `disable_fallbacks: true` 禁止跨模型回退及复用跨模型粘性目标，不统一关闭池内尝试。是否重试仍取决于入口及重放安全约束：流已输出、父响应固定归属、结果不明确的异步创建均不能跨目标重放。本地鉴权、预算和护栏拒绝不触发回退。

## 与模型、端点和会话的关系

会话缓存保存稳定 `deployment_id`、实际模型池和回退路径；每轮验证目标及路径仍合法。成功后比较更新偏好，失败不提交成功目标。通过 prompt 识别的是会话，供应商选择仍由当前路由和部署目录决定。

traffic-split 的权重作用于没有合法偏好的请求，并用于其余候选排序，不保证粘性会话每轮请求或 token 精确按比例分布。正权重变化不打散已有会话，权重归零、停用、冷却或端点不匹配会使偏好失效。其他策略不使用 allocations，停用部署应修改部署启用状态。

回退成功后可以在原请求模型/组及端点范围优先复用实际目标；撤销路径或设置 `disable_fallbacks: true` 后该跨模型偏好失效。日志继续使用原请求模型/组及会话 ID，实际执行目标单独记录。

## 策略与源码

策略：simple-shuffle/random 均匀随机；traffic-split 相对权重；least-busy 并发；latency-based-routing 延迟；cost-based-routing 输入单价；usage-based-routing token 用量。

[源码 settings.go](../../../internal/router/settings.go) · [路由组与回退](routing-groups.md)

~~~bash
go test ./internal/router -count=1
~~~
