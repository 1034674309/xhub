# Request authentication

[简体中文](readme_cn.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

At HTTP admission, resolve credentials with Resolve. For UI sessions, verify the signature before calling SessionPrincipal. Pass the resulting Principal to authz for action authorization.

| Entry point or resource | Usage and result |
| --- | --- |
| APIKeyFrom(r) | Extract request credentials without storage access |
| Resolve(ctx, cfg, db, r) | Return a Principal or an authentication/dependency error |
| SessionPrincipal(user, version, session) | Validate session state and construct a principal |

### Integration demo

Save the code as `demo/main.go` inside the repository and run `go run ./demo` from the repository root. Remove the demo directory after use.

```go
package main

import (
	"fmt"
	"github.com/sunqirui1987/xhub/internal/auth"
	"net/http/httptest"
)

// main 演示本页 API 的最小调用；无参数和返回值，打印上述预期结果，依赖与副作用见本页说明。
func main() {
	r := httptest.NewRequest("GET", "/v1/models", nil)
	r.Header.Set("Authorization", "Bearer demo-key")
	fmt.Println(auth.APIKeyFrom(r) == "demo-key")
}
```

Prints true and demonstrates extraction only. Resolve also requires loaded configuration and a real IAM store.

### Errors, state, and side effects

Preserve dependency failures as server errors rather than treating them as unknown keys. Do not log extracted plaintext credentials.

## Overview

`internal/auth` turns an HTTP credential or a stored UI session into a `Principal`. It identifies the caller; permission decisions belong to `internal/authz`. A principal preserves both the authorization identity and the key metadata needed later by routing, budget, rate-limit, spend, model-access, and request-log paths. It can represent the emergency master credential, a virtual API key, or an active signed-in user session.

The package never stores plaintext keys. `Resolve` extracts the submitted value, compares an explicitly configured master key, hashes every other value, and loads the matching IAM key row. UI sessions take a separate entry point: the gateway verifies its signed token and calls `SessionPrincipal` with the current user row, token version, and session identifier.

## External interfaces

- `APIKeyFrom(*http.Request) string` extracts one credential with fixed precedence: `x-litellm-api-key`; Bearer or Basic `Authorization`; `api-key`; then `x-api-key`. Basic credentials are base64-decoded and return the password after the first colon, or the entire decoded value when no colon exists. Only `/v1beta/models/` and `/vertex/v1/models/` additionally accept `x-goog-api-key` and then the `key` query parameter. Empty or malformed candidates fall through; the function performs no database lookup and returns an empty string when no accepted form is present.
- `Resolve(context.Context, *config.Config, *iam.DB, *http.Request) (*Principal, error)` authenticates master and virtual-key requests. Success returns a master principal or a key principal carrying the IAM row, SHA-256 lookup hash, ownership IDs, and key limits. Missing credentials, an absent store, and unknown hashes produce `invalid_api_key` wrapping `authz.ErrUnauthenticated`. Revoked, blocked/non-active, and expired rows produce `key_revoked`, `key_blocked`, or `key_expired` wrapping `authz.ErrForbidden`. Store failures remain `authz.InternalError` so the gateway returns a server failure instead of reporting a bad credential. The context controls the lookup; accepted and rejected decisions emit diagnostic logs without logging plaintext.
- `SessionPrincipal(*iam.User, version, session) (*Principal, error)` accepts only an active current user whose stored `session_version` exactly equals the signed token version. It returns a session principal with user ID, platform role, session ID, and version. A missing, disabled, deleted, or stale user returns the same unauthenticated shape as `ErrSessionInvalid()`. Password changes and other operations that advance the stored version therefore invalidate all older sessions on their next request.
- `Principal` is the handoff object. `Actor()` projects only fields used by authz; a nil receiver gives a zero actor. `IsMaster()` identifies the emergency credential. `PlatformAdmin()` is true only for an administrator session, never for a key owned by an administrator. `CanInfer()` accepts session and key kinds; key liveness is rechecked by authz before protected work. The struct also carries team/project/owner relationships, billing team and organization snapshots, selected route-template ID/source, and session information populated by gateway budget and template-resolution stages.
- `Error` wraps an authz category with a stable client code. `Code(error)` recovers that code and defaults to `invalid_api_key` for unrelated errors; callers must still test `authz.IsInternal` before turning a dependency failure into a credential response.

Primary callers are the gateway session and engine pipeline, data-plane inference admission, model-access and identity handlers, key administration, route-template selection, and limit/spend accounting. These callers enrich or consume `Principal`; they do not bypass the live authorization checks in `authz`.

## Core implementation

Credential resolution is deliberately ordered. The first non-empty accepted header wins, preventing a lower-priority compatibility header from overriding LiteLLM or `Authorization` credentials. Bearer matching is case-insensitive and trims surrounding whitespace. Basic parsing slices only after trimming the header, decodes standard base64, and splits once so passwords may contain colons. Invalid Basic data contributes no credential and allows later headers to be considered. Google query credentials are path-gated so unrelated endpoints never consume secrets from URLs.

`Resolve` first rejects an empty extraction result. It then performs an exact comparison against a non-empty configured master key; this path has no IAM row and creates only `KindMaster`. Ordinary keys require a store. Their plaintext is hashed with `iam.HashKey`, looked up by hash, and checked for lifecycle usability. Status is evaluated before expiration: a revoked key that is also past its expiry reports `key_revoked`, while any other non-active status reports `key_blocked`. Expiration is inclusive at the boundary—when current time is no longer before `ExpiresAt`, use is denied. A successful conversion copies nullable user and project IDs and retains the row pointer and hash for downstream budget and accounting state.

Authentication establishes identity but does not freeze authorization. A key accepted here is reloaded by `authz.Guard.CheckKey` so revocation, owner disablement, membership removal, team/organization blocking, or project deactivation takes effect on the next protected request. Likewise, session construction compares the signed version with the live user row. Database read errors are fail-closed as internal failures; unknown rows are intentionally indistinguishable from invalid submitted secrets.

### Test dependencies

`auth_test.go` verifies that surrounding whitespace on Basic authorization is removed before prefix slicing and decoding. `google_key_test.go` verifies Google-native header/query extraction and its path restriction. Lifecycle precedence, dependency failures, principal projection, and session invalidation are currently covered through callers and authz tests rather than direct package tests.

## Source responsibilities and entry points

- [auth.go](auth.go) is the only production file. It defines stable failure codes and the wrapping `Error` contract; the `Principal` representation and its authz/capability projections; compatibility credential extraction; virtual-key/master resolution; lifecycle classification; key-row conversion; and current-user session construction. Start at `APIKeyFrom` for wire-format precedence, `Resolve` for key authentication, and `SessionPrincipal` for UI-session invalidation.
- [auth_test.go](auth_test.go) exercises Basic credential whitespace handling.
- [google_key_test.go](google_key_test.go) exercises the Gemini/Vertex-only Google credential forms.
