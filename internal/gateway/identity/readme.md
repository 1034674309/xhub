# identity: Users, memberships and management permissions

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Responsibility

Users, memberships and management permissions. No inference forwarding; review gates before memberships, templates and mutations.

## Review order

gate.go → mount.go → handlers.go / members.go / route_template.go

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

auth/IAM supply identity and memberships; boundaries inject storage/administration.

## Verification

Adjacent tests: pagination_test.go, route_template_error_test.go

From repository root: go test ./internal/gateway/identity -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
