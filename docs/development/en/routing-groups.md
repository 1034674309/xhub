# Routing groups and fallback

Groups belong to a template. Call a group name to expand its members; call a member name to select only that model. Exact model_routes rules take precedence.

## Example

~~~json
{"routing_groups":[{"group_name":"chat-pool","models":["chat-main","chat-fast"],"routing_strategy":"least-busy"}],"fallbacks":[{"chat-pool":["chat-backup"]}]}
~~~

## Contract

This fragment belongs inside a complete [template](routing.md), not a standalone save body. Group names are 1–64 characters, cannot contain whitespace or *, cannot be default or collide with public model names. Each model belongs to at most one group per template; nested groups are rejected.

An empty fallback table inherits defaults. A source mapped to [] disables that source/category. Preserve target order; reject self-references and cycles across all fallback categories. Groups still require member-model authorization and budgets.

## Source and verification

[internal/router/groups.go](../../../internal/router/groups.go)

~~~bash
go test ./internal/router -count=1
~~~

[Test prerequisites](testing.md) · [Next: routing](routing.md)
