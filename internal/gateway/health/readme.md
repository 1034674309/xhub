# health: Liveness, readiness and console startup configuration

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Responsibility

Liveness, readiness and console startup configuration. No Server dependency, model writes or spend; liveness avoids DB; readiness failure is 503.

## Review order

health.go: Pinger → Module → healthLive / healthReady / uiConfig

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

injects only Ping and public version; PublicOrigin reads configured origin.

## Verification

Adjacent tests: health_test.go

From repository root: go test ./internal/gateway/health -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
