# VolcEngine Ark Seedance registration

[简体中文](readme_cn.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Blank-import this package at startup to register its fixed actions, then match them through registry. Administrators must still configure deployments with explicit transport/endpoint_types, base URLs, and credentials. The native data plane sends requests.

| Entry point or resource | Usage and result |
| --- | --- |
| 空白导入 volcengine | Trigger init; there is no runtime constructor |
| registry.Match(method, path) | Obtain transport, action, and path captures |

### Integration demo

Save the code as `demo/main.go` inside the repository and run `go run ./demo` from the repository root. Remove the demo directory after use.

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/provider/registry"
	_ "github.com/sunqirui1987/xhub/internal/provider/volcengine"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	hit, ok := registry.Match("POST", "/api/v3/contents/generations/tasks")
	if !ok {
		panic("registered action not found")
	}
	fmt.Println(hit.Transport.ID, hit.Action.Name)
}
```

Prints ark_contents_generation create. This only verifies protocol registration; it does not require an account or submit a task.

### Errors, state, and side effects

Ark uses Bearer and whitelists create/list/get. Creation and polling must stay on the same deployment; settlement requires measured successful results. Importing does not connect, provision credentials, or verify supplier availability. The data plane must deduplicate task settlement.

## Overview

This package declares VolcEngine as a supplier and registers Ark native asynchronous content generation. Administrators still provide the base URL, Bearer credential, and deployment prices. Two Seedance models are catalog conveniences.

### Dependencies and tests

It uses provider registration and shared Seedance billing. Tests verify supplier/model registration, prefix removal, paths, and binding without live requests.

## External interfaces

There is no exported API or handler. Init makes provider.Match recognize POST /api/v3/contents/generations/tasks, GET the collection for list, and GET /api/v3/contents/generations/tasks/{id}; dataplane native forwarding performs authentication, pinning, and settlement.

## Core implementation

ark_contents_generation uses endpoint bypass:ark-video, protocol ark, family video, Bearer auth, model field model, task field id, and strips volcengine/. Create, list, and get are separate whitelist actions. SeedanceBilling accepts only explicit successful terminal usage. Stored model IDs map to official Ark names; pricing is left to deployments.

## Source responsibilities and entry points

- [seedance.go](seedance.go): supplier, transport, three actions, two model bindings, auth/prefix/task metadata, and billing.

Verification: go test ./internal/provider/volcengine ./internal/provider -count=1. Live account access, quota, schema, and prices remain external.
