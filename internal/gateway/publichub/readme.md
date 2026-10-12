# publichub: Public model catalog, filtering, sorting and links

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Inject management authentication and link storage at startup. Public reads expose the catalog; useful-link writes still require management identity. Public catalog and user deployment discovery are separate views.

| Entry point or resource | Usage and result |
| --- | --- |
| Module(host, store) | Construct a util.Module and install via Mount/Server.Use |
| GET /public/endpoints | HTTP entry point used in this demo |

### Integration demo

Use this integration helper during startup. The host must implement the required interfaces; reg can be a gateway Server. For a standard ServeMux, adapt Handle as in the [health demo](../health/readme.md). The helper compiles but needs a complete host to execute; the HTTP demo below checks the mounted endpoint.

```go
package demo

import (
	"github.com/sunqirui1987/xhub/internal/gateway/publichub"
	"github.com/sunqirui1987/xhub/internal/util"
)

// MountDemo 演示启动时挂载模块；h 提供真实宿主能力，reg 接收路由，返回无。
// 调用方须先初始化依赖，st 提供真实存储；挂载会修改路由表，不执行 HTTP 请求。
func MountDemo(h publichub.Host, st publichub.Store, reg util.Registrar) {
	publichub.Module(h, st).Mount(reg)
}
```

Start a configured local gateway first. See [wire.go](../wire.go) and [routes.go](../routes.go) for complete host wiring; do not substitute nil dependencies.

Run the following commands from the repository root.

```sh
curl -sS -i "http://localhost:4000/public/endpoints"
```

Returns the public endpoint catalog without credentials. These are declarable endpoints, not proof of callable deployments.

### Errors, state, and side effects

Public responses must exclude credentials and tenant configuration. Do not confuse market catalogs with deployed models. Link updates persist data and require management authorization.

## Responsibility

Public model catalog, filtering, sorting and links. No credential disclosure; handles pagination, case-folded filtering and store failures.

## Review order

public_hub.go: Module → publicModelHub / publicModelHubFacet → filterPublicModels / sortPublicModels

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

catalog supplies public data; minimal Host gates writes; Store persists link KV.

## Verification

Adjacent tests: public_hub_test.go

From repository root: go test ./internal/gateway/publichub -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
