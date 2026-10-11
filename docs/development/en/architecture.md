# Request lifecycle and module boundaries

~~~text
Console :3000 ─┐
Application ──┴─> Gateway :4000
                  -> Auth / Authz / Limits
                  -> Router -> Dataplane -> Provider
                  -> Usage -> PostgreSQL
                           -> Redis queue (optional)
~~~

## Follow one request

| Stage | Source | Boundary |
| --- | --- | --- |
| HTTP wiring | [gateway](../../../internal/gateway/readme.md) | UI buttons do not authorize requests |
| Identity / authorization | [auth](../../../internal/auth/auth.go), [authz](../../../internal/authz/decide.go) | Client metadata is not trusted identity |
| Candidate compilation | [router](../../../internal/router/readme.md) | Cannot expand model scope |
| Upstream execution | [dataplane](../../../internal/dataplane/readme.md) | Cannot splice another provider after a partial stream |
| Protocol registration / conversion | [provider](../../../internal/provider/readme.md), [llm](../../../internal/llm/readme.md) | A catalog entry does not prove executable support |
| Configuration / identity / accounting | [store](../../../internal/store), [iam](../../../internal/iam/readme.md) | Current configuration must not rewrite historical ownership |

Normal inference runs authentication, limits, guardrails, routing, execution and accounting. Native tasks use a separate lifecycle; chat cache and retry rules do not apply automatically.

## Verification

~~~bash
go test ./internal/router ./internal/dataplane/... -count=1
go test ./internal/gateway -run 'TestVisibilityChain' -count=1
~~~

[Runtime boundaries](runtime.md) · [Pricing](pricing.md) · [Test environment](testing.md)
