# Routing templates

Save a complete template, then bind it to a key, team or organization. Resolution selects the first whole template in key → team → organization → platform order; it does not merge fields.

## Example

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

## Contract

| Field | Behavior |
| --- | --- |
| model_routes | Exact public model rules; strategy required |
| allocations omitted | Inherit model default weights |
| allocations: [] | All compatible deployments use weight 1 |
| explicit weight 0 | Exclude the deployment |
| max_attempts | Attempts per deployment, including the first |
| timeout_seconds | Timeout per attempt |
| failure_threshold: 0 | Disable passive cooldown |
| cooldown_seconds: 0 | Use 60 seconds |

All four retry_policy fields are required. Reject unknown fields, invalid references, duplicate models, negative/nonfinite weights and fallback cycles. Streaming output prevents switching providers. Ambiguous task creation must not be repeated.

## Source and verification

[internal/router/settings.go](../../../internal/router/settings.go)

~~~bash
go test ./internal/router -count=1
~~~

[Test prerequisites](testing.md) · [Next: routing-groups](routing-groups.md)
