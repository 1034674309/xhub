# Backend module map

[简体中文](README.zh-CN.md) · [Feature implementation reference](../docs/development/en/implementation.md)

## Overview

This tree implements identity, the management control plane, inference execution, routing, pricing, and persistence. Command packages compose the services; [cmd/regression](../cmd/regression/readme.md) contains business regression tests and their shared fixtures. Exported Go symbols are implementation seams inside the repository; HTTP contracts and configuration documents are the client boundary.
Inference resolves a session or virtual key, reloads ownership, checks action permissions, model access, budgets, and limits, selects one routing document, and executes an eligible deployment. Successful usage is priced with a recorded rate snapshot and historical ownership. Cache and affinity are committed only after success.
iam owns identity and accounting; store owns framework configuration and deployments; live owns Redis hot state; util supplies response caching, logging, and pagination helpers. Scope totals are different views of one charge. Never concatenate a second upstream response after stream output or acknowledge a queue item before durable commit.

This is the organizational root of backend implementation, with no production Go file or independent Go package. Children follow business responsibilities; each README describes its own directory. Exported symbols support repository composition rather than providing an external SDK.

## External interfaces

### Process and HTTP boundary

Production starts in [cmd/gateway/main.go](../cmd/gateway/main.go). It reads -config (default configs/config.yaml) and -addr (default :4000), calls config.Load, iam.Open/Migrate, store.Open, and gateway.New, then listens through Server.Run. Configuration, database connection, and migration errors abort startup; listener failure terminates the process. The root mounts no HTTP routes: contracts come from gateway modules, catalog wiring, and registered provider transports.

Server.Handler returns the http.Handler used by embedding hosts and tests; Handle installs routes and Use installs modules. Management calls require sessions and action authorization. Inference accepts eligible sessions or virtual keys; master credentials have limited bootstrap/emergency permissions. Fields, authentication, and status codes belong to [API documentation](../docs/api/en/models.md) and module READMEs. A catalog path does not establish business support.

### Package collaboration contracts

httpx.Module/Registrar define module installation. auth.Principal and authz.Actor/Object/Scope connect identity, action decisions, and query filters. dataplane.Adapted, Bypass, and Runtime host interfaces inject identity, storage, routing, and logging into execution; gateway children call Host/Gate interfaces to avoid cycles. config.ModelEntry, provider.Transport, and router policies connect deployments to execution. Owning packages document inputs, return values, errors, and side effects.

## Core implementation

### Startup assembly

IAM opens PostgreSQL and migrates identity/accounting tables; Store manages framework configuration and model resources. gateway.New creates authorization, HTTP client, response cache, in-flight counters, and extension registry. It attempts configured Redis, overlays persisted settings, loads deployments and price overrides, starts scheduled price refresh, and seeds an initial administrator. Explicit modules mount before catalog routes; later registrations cannot replace an existing path. provider/all imports trigger provider initialization, and deployment configuration must select matching capabilities and transports.

### Request chain and failures

HTTP entry resolves credentials and sessions, verifies current state, authorizes actions against trusted ownership, and checks model access, budgets, and limits. Requests select the nearest complete template in the key→team→organization chain or builtin model allocation. The data plane coordinates bodies, guardrails, extensions, capability filters, disabled/cooling deployments, candidate order, cache, and sending. Dialogue conversion is used where required; native transports preserve native fields for registered actions. Retry, fallback, and timeout handling cannot append another provider response after stream output.

Asynchronous tasks retain the creating deployment and call context. Polls return there, and valid measured usage at terminal success permits settlement. Ambiguous creation results do not justify blind replay. Runtime logs, request logs, and persistent audit serve different purposes; redaction does not replace read authorization.

### State and persistence

IAM transactions retain events, ownership, and rate snapshots and prevent duplicate settlement through request/task identifiers. User, key, team, project, and organization totals are views of one expense and must not be summed. Store KV is not an identity authorization store. Redis holds hot spend, limits, cooldowns, affinity, and queues; consumers commit before acknowledging. Process/synchronous paths without Redis do not provide shared multi-instance guarantees. Response caching stores complete successful bytes under identity/configuration-aware keys.

## Source responsibilities and entry points

### Child directory index

| Directory | Responsibility |
| --- | --- |
| [auth](auth/readme.md) | Caller authentication |
| [authz](authz/readme.md) | Action authorization and query scopes |
| [catalog](catalog/readme.md) | Model catalog, rates, and normalized usage |
| [config](config/readme.md) | Startup configuration and model definitions |
| [dataplane](dataplane/readme.md) | Inference execution and settlement |
| [gateway](gateway/readme.md) | HTTP gateway and module composition |
| [hooks](hooks/readme.md) | In-flight request accounting |
| [httpx](httpx/readme.md) | HTTP errors and module contracts |
| [iam](iam/readme.md) | Identity resources and transactional accounting |
| [live](live/readme.md) | Redis hot state and usage queue |
| [llm](llm/readme.md) | Upstream construction and protocol adaptation |
| [plugin](plugin/readme.md) | Upstream extension registry |
| [provider](provider/readme.md) | Provider capabilities and transports |
| [router](router/readme.md) | Deployment filtering, ordering, and weighted split |
| [store](store/readme.md) | Framework configuration and deployment storage |
| [util](util/readme.md) | Response caching, runtime logging and redaction, and overflow-safe pagination |

### Reading and verification

Follow startup and HTTP through cmd/gateway/main.go → gateway/server.go, routes.go, and engine.go; inference through gateway/ingress.go and bypass.go → dataplane; durable settlement through iam/schema.sql, usage.go, and task_usage.go. Every child README inventories local files and concrete entry points.

Deterministic checks: go test ./internal/... -count=1. Real backend contracts are covered in [cmd/regression](../cmd/regression/readme.md); browser flows use cd frontend && npm run test:e2e. For database acceptance inspect XHUB_REGRESSION_STRICT=1 and skips. Redis and live providers require their own environments. Unit tests do not establish live connectivity, paid task completion, or actual billing accuracy. Documentation stores no deployment credentials.
