# health: Liveness, readiness and console startup configuration

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Inject a non-nil Pinger and public version, then mount the Module on a Registrar. Liveness checks the process, readiness invokes Ping, and version details do not perform the same database probe.

| Entry point or resource | Usage and result |
| --- | --- |
| Module(db, version, sso...) | Return liveness, readiness, and startup configuration routes |
| PublicOrigin() | Use the environment origin or localhost:4000 |

### Integration demo

Save the code as `demo/main.go` inside the repository and run `go run ./demo` from the repository root. Remove the demo directory after use.

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/gateway/health"
	"net/http"
	"net/http/httptest"
)

type demoPinger struct{}

// Ping 模拟健康依赖；无参数，返回 nil，仅供无网络模块演示。
func (demoPinger) Ping() error { return nil }

type registrar struct{ mux *http.ServeMux }

// Handle 将模块路由适配到 ServeMux；参数为路径与处理器，无返回值，修改本机路由表。
func (r registrar) Handle(pattern string, handler http.HandlerFunc) {
	r.mux.HandleFunc(pattern, handler)
}

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	mux := http.NewServeMux()
	health.Module(demoPinger{}, "demo-version").Mount(registrar{mux})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/health/readiness", nil))
	fmt.Println(w.Code)
}
```

Prints 200; a Pinger returning an error produces 503. This mounting demo uses a fixture; real readiness needs the real database.

### Errors, state, and side effects

Module accesses db.Ping during construction and cannot accept nil. util.Registrar.Handle takes HandlerFunc, so adapt ServeMux as shown. Do not use details as a readiness probe.

## Responsibility

Liveness, readiness and console startup configuration. No Server dependency, model writes or spend; liveness avoids DB; readiness failure is 503.

## Review order

health.go: Pinger → Module → healthLive / healthReady / uiConfig

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

injects only Ping and public version; PublicOrigin reads configured origin.

## Verification

Adjacent tests: health_test.go

From repository root: go test ./internal/gateway/health -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
