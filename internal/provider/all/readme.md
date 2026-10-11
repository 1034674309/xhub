# all

Startup composition of all vendors.

Code entry points: [all.go](all.go)

This package only imports vendor packages for startup registration. It has no execution or conversion logic to unit test. Vendor behavior is tested beside its source; registry contracts are tested in [registry](../registry/readme.md).

Compile the composition with `go test ./internal/provider/all/...`.

Backend regressions live in cmd/regression; browser flows in frontend/e2e. Offline tests use local HTTP upstreams and do not establish production credential or vendor availability.
