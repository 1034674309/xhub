# usage: Usage normalization, missing-count estimation and first-token timing

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Responsibility

Usage normalization, missing-count estimation and first-token timing. Distinguish explicit zero from absence; estimates are not upstream measurements and do not define prices.

## Review order

usage.go: UsageCounts / UsageFromDocument / MergeUsage → CompleteUsage / TtftMillis

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

stream, dialogue and execution consume pure helpers; catalog and gateway/billing own prices.

## Verification

Adjacent tests: metrics_test.go, usage_stream_test.go

From repository root: go test ./internal/dataplane/usage -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
