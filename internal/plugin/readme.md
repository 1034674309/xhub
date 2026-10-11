# Pre-upstream extension registry

## Overview

The plugin package defines an in-process extension chain that runs immediately before an inference request can use the response cache or contact an upstream provider. It separates extension implementations from the gateway process through a small named interface. It is not a dynamic loader: there is no script execution, package discovery, installation endpoint, persistence, or process isolation.

## External interfaces

- `Call` is immutable call context supplied by the data plane: operation, public model, and request path. It carries no request body or credential.
- `Decision` contains allow/refuse state, HTTP status, stable error code, client message, and response headers. A zero decision allows processing to continue. If `Refuse` is true, the data plane skips cache lookup and upstream dispatch and writes the protocol response.
- `Extension` requires a stable non-empty `Name()` and `BeforeUpstream(Call) Decision`. Implementations execute synchronously and can have their own side effects; the registry cannot roll them back.
- `New() *Registry` returns an empty registry; the zero-value Registry is also usable.
- `Register(ext) error` rejects a nil registry, nil extension, empty name, or duplicate name. On error, order and lookup state remain unchanged. Successful registration appends to deterministic order.
- `Names() []string` returns a copy in registration order; a nil registry returns nil.
- `Invoke(name, call) (Decision, error)` executes exactly the named extension. Missing names, including calls on a nil registry, return an error and do not invoke another extension.
- `Run(call) Decision` executes a snapshot in registration order. Headers merge with later values replacing earlier values. The first refusal stops iteration and returns its status/code/message plus all headers accumulated through that extension. An empty or nil registry allows the call.

internal/gateway constructs and registers extensions; internal/dataplane runs them. No HTTP route is registered here.

## Core implementation

Registry protects the ordered extension slice, parallel name slice, and name map with one mutex. Register mutates all three while holding the lock. Names copies under lock. Invoke fetches one implementation under lock and calls it after unlocking. Run likewise snapshots the ordered slice before callbacks, so callbacks cannot deadlock by registering and registrations made during a run affect only later runs.

Header accumulation uses a fresh map. A refusal preserves earlier headers and lets the refusing extension overwrite a duplicate. Non-refusing status, code, and message fields are ignored; after all extensions allow, only merged headers are returned. Extensions are responsible for thread safety, bounded latency, deterministic policy, and avoiding irreversible work or secret disclosure.

### Test dependencies

`registry_test.go` covers zero-value use, validation, stable ordering, copied names, exact lookup, header precedence, and first-refusal stopping. Run `go test ./internal/plugin -count=1`; no external service is required.

## Source responsibilities and entry points

- `registry.go` contains all production contracts and synchronization: Call, Decision, Extension, Registry, New, Register, Names, Invoke, and Run.
- `registry_test.go` supplies deterministic test extensions and verifies registry and chain behavior.
