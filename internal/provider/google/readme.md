# google

Gemini and Vertex registration.

Code entry points: [native.go](native.go)

Unit tests live beside source: [native_test.go](native_test.go)

go test ./internal/provider/google/...

Backend regressions live in cmd/regression; browser flows in frontend/e2e. Offline tests use local HTTP upstreams and do not establish production credential or vendor availability.
