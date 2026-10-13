# Route-template compilation and deployment scheduling

[简体中文](readme_cn.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Resolve the template for the identity scope, compile it and check Err, then schedule candidates using the request runtime snapshot. Attempt returned deployments in order and report the actual successful target to affinity and accounting.

| Entry point or resource | Usage and result |
| --- | --- |
| SelectRouteTemplate / Compile | Select and validate request routing settings |
| Schedule(pool, alias, strategy, state, pin) | Return ordered deployment candidates |
| DeploymentID / CooldownID | Distinguish stable deployment and runtime cooldown identities |

### Integration demo

Save the code as `demo/main.go` inside the repository and run `go run ./demo` from the repository root. Remove the demo directory after use.

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/config"
	"github.com/sunqirui1987/xhub/internal/router"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	models := []config.ModelEntry{{ModelName: "demo-model", LiteLLMParams: map[string]any{"model": "upstream-model"}}}
	pool := router.Schedule(models, "demo-model", "random", router.State{}, "")
	fmt.Println(len(pool), pool[0].ModelName)
}
```

Prints 1 demo-model and demonstrates scheduling one candidate. Production still requires capability, authorization, budget, and template checks.

### Errors, state, and side effects

A pin cannot bypass disabled, cooled-down, or zero-weight deployments. Group members retain real identities for authorization and pricing. Do not ignore Compile result Err.

## Overview

This package converts persisted routing configuration and the current model directory into an immutable per-request RouteSettings snapshot, then selects an ordered deployment attempt list. It owns model matching, explicit groups, relative traffic weights, retry timing values, three fallback categories, scope inheritance, cooldown filtering, sticky preference, and strategy algorithms. It does not register HTTP, authenticate, call providers, record metrics, or persist configuration.

Exact public model names win before wildcard patterns. Wildcards are ordered by specificity and rewrite the upstream model using captured groups. Disabled deployments are excluded. All functions return new slices/maps/documents or read-only results unless a caller explicitly stores them.

### Dependencies and tests

It uses config model entries, catalog price resolution, IAM route-template rows, llm wildcard helpers, and logging. Unit/regression tests cover allocations, wildcard matching, groups, fallback graphs, template compilation/validation, cleanup, scope resolution, strategies, sticky scheduling, cooldown identities, and current time-window prices.

## External interfaces

The package registers no handler. Gateway identity exposes route-template management under /route_template/..., routing groups under /routing/groups, and settings under /router/settings; those handlers call these Go APIs. Dataplane calls compiled RouteSettings and Schedule for inference.

Compile(selected, store, models) loads model defaults and explicitly configured template policies, validates references and returns errors in RouteSettings.Err without writes. iam.SelectRouteTemplate chooses the first binding from key, team and organization; an absent selected document falls back to builtin. Builtin settings use one attempt, 60 seconds, threshold three and no cooldown.

Parse/validation APIs cover FallbackPolicy, Group, Policy, default weights, template documents, catalogs, strategies, and deployment ownership. RouteSettings.ForModel resolves an exact model rule, otherwise a group, otherwise model defaults; Candidates expands only explicit group names; Schedule applies shares and optional pin. Retries, TimeoutSeconds, AllowedFails, and CooldownSeconds expose validated execution values. All, Available, Order, Pick, DeploymentID, and CooldownID implement scheduling. CleanAllocations and CleanTemplateWeights deep-clean stale deployment references.

## Core implementation

Matching first selects exact aliases. If absent, it sorts wildcard patterns by literal length and complexity, matches safely compiled regular expressions, and substitutes captured values into the upstream model. Groups preserve each member's real model identity for authorization and billing. DeploymentID prefers deployment_id then model_info.id. CooldownID combines api_base and model with stable identity preferring pricing_id, deployment_id, then model_info.id, plus credential name to isolate accounts.

traffic-split independently draws from positive finite relative weights; omitted deployments default to weight one, explicit zero and cooled entries are excluded. random and simple-shuffle use uniform random choice. least-busy, cost-based-routing, latency-based-routing, usage-based-routing, and tag-based-routing compare runtime maps or configured fallback values. Cost routing uses the active rate table before flat price and treats unpriced/invalid values as positive infinity. A pin moves a deployment first only if it remains matched, healthy, and positive-weight.

Template parsing rejects unknown fields, duplicate model rules/groups, invalid retry numbers, allocations on non-split strategies, group overlap, dangling deployment IDs, unknown fallback nodes, overlong/duplicate targets, and cycles across all three fallback categories. Compile validates both local and merged global graphs. Cleanup deep-copies JSON, removes dangling allocations, drops weights for zero/one deployment, and restores default equal behavior when a complete remaining allocation would be all zero.

## Source responsibilities and entry points

- [compile.go](compile.go): ConfigReader and per-request merge/validation of defaults, groups, and fallbacks.
- [fallback.go](fallback.go): strict FallbackPolicy parsing, category targets, and cross-category graph validation.
- [groups.go](groups.go): Group/GroupArgs parsing, naming/member/allocation validation, and candidate expansion.
- [policy.go](policy.go): Allocation, Policy, DefaultWeights, strict parsing, shares, and deployment ownership.
- [router.go](router.go): State, exact/wildcard matching, all strategy algorithms, weighted draw, runtime identities, pricing comparison, and strategy validation.
- [schedule.go](schedule.go): health/weight filtering and compatible sticky-first attempt ordering.
- [settings.go](settings.go): RouteSettings, scope resolution, builtin defaults, model precedence, retry accessors, and strict document fields.
- [template.go](template.go): routing-group and three-category fallback extraction plus live-catalog validation.
- [weight_cleanup.go](weight_cleanup.go): stale allocation cleanup and deep-copied template repair.

Verification: go test ./internal/router -count=1. Redis freshness, concurrent live load, provider health, and real price-feed correctness remain external.
