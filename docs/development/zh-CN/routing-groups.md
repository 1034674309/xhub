# 路由组与回退

组随模板保存，不独立提交历史组接口。调用组名展开成员部署；调用成员名只选该模型，精确 model_routes 规则优先。

以下片段放进[完整模板](routing.md)：

~~~json
{"routing_groups":[{"group_name":"chat-pool","models":["chat-main","chat-fast"],"routing_strategy":"least-busy"}],"fallbacks":[{"chat-pool":["chat-backup"]}]}
~~~

| 约束 | 规则 |
| --- | --- |
| 组名 | 1–64 字符；无空白或 *；不能为 default 或公开模型名 |
| 成员 | 非空、确切模型名；不允许嵌套；每模板每模型最多一组 |
| 三类回退 | 通用 / 上下文 / 内容策略分别保存有序目标 |
| 空表 [] | 继承模型默认回退 |
| 源映射到 [] | 禁用该源、该类别回退 |
| 循环 | 默认和模板合并后统一判环 |

组不能扩大成员模型权限或预算。本地护栏拦截不触发供应商内容策略回退。

[源码 groups.go](../../../internal/router/groups.go)

~~~bash
go test ./internal/router -count=1
~~~
