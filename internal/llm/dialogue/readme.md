# dialogue

Unified dialogue codecs and strict validation.

Code entry points: [dialogue_google.go](dialogue_google.go), [model.go](model.go), [request.go](request.go), [response.go](response.go), [usage.go](usage.go), [validation.go](validation.go)

Unit tests live beside source: [dialogue_google_test.go](dialogue_google_test.go), [dialogue_sdk_test.go](dialogue_sdk_test.go), [dialogue_test.go](dialogue_test.go)

go test ./internal/llm/dialogue/...

model defines normalized values; request/response encode and decode; validation checks fields; usage normalizes reported facts; dialogue_google handles Google. No network calls.

Backend regressions live in cmd/regression; browser flows in frontend/e2e. Offline tests use local HTTP upstreams and do not establish production credential or vendor availability.
