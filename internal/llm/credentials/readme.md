# credentials

Trusted credential hydration.

Code entry points: [credential.go](credential.go)

Unit tests live beside source: [hydrate_test.go](hydrate_test.go)

go test ./internal/llm/credentials/...

Backend regressions live in cmd/regression; browser flows in frontend/e2e. Offline tests use local HTTP upstreams and do not establish production credential or vendor availability.
