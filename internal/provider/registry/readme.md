# registry

Built-in registration and path matching.

Code entry points: [catalog_binding.go](catalog_binding.go), [match.go](match.go), [models.go](models.go), [registry.go](registry.go), [suppliers.go](suppliers.go)

Unit tests live beside source: [catalog_binding_test.go](catalog_binding_test.go), [endpoint_redesign_test.go](endpoint_redesign_test.go), [isolation_test.go](isolation_test.go), [match_test.go](match_test.go), [registry_test.go](registry_test.go)

go test ./internal/provider/registry/...

Registration and reads isolate mutable slices, maps and billing callback containers. Match accepts only built-in actions, never deployment-supplied paths.

Backend regressions live in cmd/regression; browser flows in frontend/e2e. Offline tests use local HTTP upstreams and do not establish production credential or vendor availability.

## Coverage gate

Run scripts/core-codec-coverage.sh using bash. Registry and dialogue codec must each reach 95% statement coverage. Test failures, missing evidence, or a lower percentage fail the command. The profile uses package unit tests without adding backend regression or browser coverage.
