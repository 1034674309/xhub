# capability

Public endpoints and protocol matching.

Code entry points: [capability.go](capability.go), [endpoints.go](endpoints.go), [execution.go](execution.go)

Unit tests live beside source: [capability_test.go](capability_test.go)

go test ./internal/provider/capability/...

Backend regressions live in cmd/regression; browser flows in frontend/e2e. Offline tests use local HTTP upstreams and do not establish production credential or vendor availability.
