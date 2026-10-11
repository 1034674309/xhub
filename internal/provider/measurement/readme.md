# measurement

Response/SSE usage and task billing facts.

Code entry points: [fal_billing.go](fal_billing.go), [native_usage.go](native_usage.go), [seedance_billing.go](seedance_billing.go)

Unit tests live beside source: [bindings_test.go](bindings_test.go), [google_usage_test.go](google_usage_test.go), [native_usage_test.go](native_usage_test.go)

go test ./internal/provider/measurement/...

Backend regressions live in cmd/regression; browser flows in frontend/e2e. Offline tests use local HTTP upstreams and do not establish production credential or vendor availability.
