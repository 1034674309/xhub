# deployment

Explicit bindings, validation and candidates.

Code entry points: [bindings.go](bindings.go), [candidates.go](candidates.go), [catalog_binding.go](catalog_binding.go), [execution.go](execution.go), [registry.go](registry.go), [validate.go](validate.go)

Unit tests live beside source: [bindings_test.go](bindings_test.go), [candidates_test.go](candidates_test.go), [catalog_binding_test.go](catalog_binding_test.go), [endpoint_redesign_test.go](endpoint_redesign_test.go), [registry_test.go](registry_test.go), [validate_test.go](validate_test.go)

go test ./internal/provider/deployment/...

Backend regressions live in cmd/regression; browser flows in frontend/e2e. Offline tests use local HTTP upstreams and do not establish production credential or vendor availability.
