# Qiniu Fal and content-generation catalog

[简体中文](readme_cn.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Blank-import this package at startup to register its fixed actions, then match them through registry. Administrators must still configure deployments with explicit transport/endpoint_types, base URLs, and credentials. The native data plane sends requests.

| Entry point or resource | Usage and result |
| --- | --- |
| 空白导入 qiniu | Trigger init; there is no runtime constructor |
| registry.Match(method, path) | Obtain transport, action, and path captures |

### Integration demo

Save the code as `demo/main.go` inside the repository and run `go run ./demo` from the repository root. Remove the demo directory after use.

```go
package main

import (
	"fmt"
	_ "github.com/sunqirui1987/xhub/internal/provider/qiniu"
	"github.com/sunqirui1987/xhub/internal/provider/registry"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	hit, ok := registry.Match("POST", "/v3/contents/generations/tasks")
	if !ok {
		panic("registered action not found")
	}
	fmt.Println(hit.Transport.ID, hit.Action.Name)
}
```

Prints qiniu_contents_generation create. This only verifies protocol registration; it does not require an account or submit a task.

### Errors, state, and side effects

Content generation uses Bearer; Fal queue uses Key. Content generation registers create/get only, without list. Importing does not connect, provision credentials, or verify supplier availability. The data plane must deduplicate task settlement.

## Overview

This package registers Qiniu-compatible Fal queue transports and the Qiniu /v3 content-generation transport. Connections are administrator supplied through custom/OpenAI-compatible deployments; registration creates no account and reads no credential. Model rows keep upstream IDs, pricing IDs, sources, transport IDs, and billing dimensions separate.

### Dependencies and tests

It uses provider registration and shared Fal/Seedance billing. Tests cover exact paths, auth declarations, prefix handling, measured units, success boundaries, variants, and incomplete-context pricing blocks.

## External interfaces

There is no exported function or local router. Fal exposes exact POST /queue/{model-path}, GET /queue/{group}/requests/{request_id}, and GET .../status actions. Content generation exposes POST /v3/contents/generations/tasks and GET /v3/contents/generations/tasks/{id}, with no list action. provider.Match dispatches them to dataplane native handling.

## Core implementation

registerFal creates one transport per queue family, two shared follow-up actions, and one create action per concrete model path. It uses Authorization: Key, request_id, strips qiniu/, rewrites queue URLs, and binds FalBilling to resolution/audio/variant rules. Init explicitly enumerates Seedance/Dreamina, Kling, Vidu, Veo, and MiniMax variants, so no wildcard expands the whitelist.

The content transport uses Bearer auth, task field id, prefix stripping, and SeedanceBilling. Four bytedance Seedance models carry distinct marketplace pricing IDs and sources. Settlement later requires explicit terminal success and positive measured usage.

## Source responsibilities and entry points

- [fal.go](fal.go): falEndpoint, registerFal, concrete queue actions/models, catalog rows, Key auth, URL rewriting, and Fal billing bindings.
- [seedance.go](seedance.go): qiniu_contents_generation create/get, four models, Bearer auth, prefix handling, and Seedance billing.

Verification: go test ./internal/provider/qiniu ./internal/provider -count=1. Availability, quota, duration, and price freshness remain external.
