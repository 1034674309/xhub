# Route-template compilation and deployment scheduling

[简体中文](readme_cn.md)

## Overview

This package converts persisted routing configuration and the current model directory into an immutable per-request RouteSettings snapshot, then selects an ordered deployment attempt list. It owns model matching, explicit groups, relative traffic weights, retry timing values, three fallback categories, scope inheritance, cooldown filtering, sticky preference, and strategy algorithms. It does not register HTTP, authenticate, call providers, record metrics, or persist configuration.

Exact public model names win before wildcard patterns. Wildcards are ordered by specificity and rewrite the upstream model using captured groups. Disabled deployments are excluded. All functions return new slices/maps/documents or read-only results unless a caller explicitly stores them.

### Dependencies and tests

It uses config model entries, catalog price resolution, IAM route-template rows, llm wildcard helpers, and logging. Unit/regression tests cover allocations, wildcard matching, groups, fallback graphs, template compilation/validation, cleanup, scope resolution, strategies, sticky scheduling, cooldown identities, and current time-window prices.

## External interfaces

The package registers no handler. Gateway identity exposes route-template management under /route_template/..., routing groups under /routing/groups, and settings under /router/settings; those handlers call these Go APIs. Dataplane calls compiled RouteSettings and Schedule for inference.

Compile(selected, store, models) loads model_defaults, legacy groups when the template omits routing_groups, and model_fallbacks; overlays only explicitly present template fallback categories; validates all references and the merged graph; and returns errors in RouteSettings.Err without writes. Resolve selects key, then team, then organization binding; a bound ID whose row is missing stops inheritance and falls back to builtin. RequestChain constructs that order. BuiltinDocument/BuiltinSettings provide one attempt, 60 seconds, threshold three, no cooldown, and model-default traffic split.

Parse/validation APIs cover FallbackPolicy, Group, Policy, default weights, template documents, catalogs, strategies, and deployment ownership. RouteSettings.ForModel resolves model rule, group, legacy template default, then model default; Candidates expands only explicit group names; Schedule applies shares and optional pin. Retries, TimeoutSeconds, AllowedFails, and CooldownSeconds expose validated execution values. All, Available, Order, Pick, DeploymentID, and CooldownID implement scheduling. CleanAllocations and CleanTemplateWeights deep-clean stale deployment references.

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
