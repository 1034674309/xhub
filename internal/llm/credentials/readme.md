# credentials

Trusted credential hydration.

Named credentials supply only the fields consumed by the execution layer: `api_key`, `api_base`, and `custom_llm_provider`. A nonblank credential key replaces the deployment key; other fields fill missing values. Environment references are resolved at hydration time. Existing deployment parameters are preserved.

Azure, AWS, and Vertex automatic authentication fields are not imported from named credentials because this layer has no corresponding authentication executor. Guardrail AWS configuration belongs to its own module.

Code entry points: [credential.go](credential.go)

Unit tests live beside source: [hydrate_test.go](hydrate_test.go)

go test ./internal/llm/credentials/...

Backend regressions live in cmd/regression; browser flows in frontend/e2e. Offline tests use local HTTP upstreams and do not establish production credential or vendor availability.
