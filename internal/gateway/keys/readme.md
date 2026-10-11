# keys: Platform and personal API key lifecycle

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Responsibility

Platform and personal API key lifecycle. No inference; review creation, revocation, expiry, scope and secret disclosure together.

## Review order

host.go → mount.go → generate.go → admin.go / personal.go

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

Host injects identity/storage; auth/IAM own hashes and scope.

## Verification

Adjacent tests: key_input_test.go, personal_test.go

From repository root: go test ./internal/gateway/keys -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
