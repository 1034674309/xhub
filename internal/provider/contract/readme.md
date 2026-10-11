# contract

Shared value contracts.

Code entry points: [type.go](type.go)

Unit tests live beside source: [match_test.go](match_test.go)

go test ./internal/provider/contract/...

Backend regressions live in cmd/regression; browser flows in frontend/e2e. Offline tests use local HTTP upstreams and do not establish production credential or vendor availability.
