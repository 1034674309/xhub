# templateauth: Route-template selection authorization

[中文](readme_cn.md) · [Parent architecture](../readme.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Before saving keys, teams, organizations, or template bindings, call Selection to verify template visibility. Mutate the scope only after success. An empty ID means inheritance.

| Entry point or resource | Usage and result |
| --- | --- |
| Host.Identity / Host.Authorize | Read trusted template ownership and authorize |
| Selection(host, request, principal, id) | Return errors; empty IDs allow inheritance |

### Integration demo

This is a Go integration helper for a host. It compiles as a separate package inside the repository; callers must supply the real dependencies described below before execution.

```go
package demo

import (
	"github.com/sunqirui1987/xhub/internal/auth"
	"github.com/sunqirui1987/xhub/internal/gateway/templateauth"
	"net/http"
)

// ValidateSelection 演示保存前的检查；传入真实宿主、请求、已鉴权身份及模板 ID，返回错误，不执行保存。
func ValidateSelection(h templateauth.Host, r *http.Request, p *auth.Principal, templateID string) error {
	return templateauth.Selection(h, r, p, templateID)
}
```

The helper compiles; nonempty IDs require real IAM. nil permits the caller to continue with its save; Selection does not save.

### Errors, state, and side effects

Missing templates return NotFound, lookup failures return InternalError, and invisible templates return authorization errors. Do not trust client-supplied organization/team ownership.

## Responsibility

Route-template selection authorization. No template routing; enforce authorization independently of frontend hiding.

## Review order

selection.go: exported validators → callers

Read injected interfaces or input contracts first, then entry points, helpers and adjacent tests. Inspect parent callers when changing request lifecycle behavior.

## Dependencies

authz/IAM and injected lookups constrain selection.

## Verification

Adjacent tests: selection_test.go

From repository root: go test ./internal/gateway/templateauth -count=1

Real HTTP/persistence contracts belong in cmd/regression; browser → gateway → local upstream flows belong in frontend/e2e. Private schemas and temporary fixtures are cleaned up. Offline tests do not assume paid vendor credentials.
