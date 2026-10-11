# all

Startup composition of all vendors.

Code entry points: [all.go](all.go)

Unit tests live beside source:

go test ./internal/provider/all/...

Backend regressions live in cmd/regression; browser flows in frontend/e2e. Offline tests use local HTTP upstreams and do not establish production credential or vendor availability.
