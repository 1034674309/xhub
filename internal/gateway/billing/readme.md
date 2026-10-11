# billing: Deployment rate selection and per-call pricing

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Responsibility

Deployment rate selection and per-call pricing. No body parsing, budget deduction or persistence; missing manual prices must not silently fall back.

## Review order

cost.go: DeploymentFinder → CallCost → deploymentCost

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

only deployment lookup; catalog owns window/cache/image/video arithmetic; spend.go persists results.

## Verification

Adjacent tests: cost_test.go

From repository root: go test ./internal/gateway/billing -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
