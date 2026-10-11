# In-flight request hooks

## Overview

The hooks package tracks active inference calls by virtual-key ID inside one gateway process. Gateway startup creates an Engine and the data plane calls Begin when execution starts, then defers the returned release function across normal completion, timeout, send failure, and stream interruption. The count is transient concurrency state. It does not grant access, enforce a cluster-wide limit, persist accounting, or coordinate gateway replicas.

## External interfaces

- `New() *Engine` returns an Engine with an initialized empty counter map. Gateway server assembly is the primary constructor call site.
- `(*Engine).Begin(keyID string) func()` reserves one local in-flight slot for a non-empty key and returns a release closure. An empty ID produces a safe no-op closure and does not create map state. The closure returns no error and is idempotent: its first call decrements the exact key captured at Begin time, removes the entry when the count reaches zero, and subsequent calls do nothing.

The package exposes no count-reading method and registers no route. internal/dataplane uses the lifecycle hook; internal/gateway owns the Engine. A caller must always invoke the returned closure, normally with defer.

## Core implementation

Engine contains one mutex and a map from key ID to integer count. Begin locks only while incrementing, then creates a closure containing its own sync.Once. On the first release, that Once enters another short critical section, decrements the same map entry, and deletes it at zero or below. Deletion prevents inactive key IDs from accumulating. Per-closure Once is essential: without it, a duplicated cleanup path could decrement a different call's slot or remove a key while another call is still running.

The mutex gives consistent updates among concurrent calls in one process. No condition variable or admission decision exists here, and no public reader consumes the count in this version; the structure preserves accurate lifecycle state for local routing/concurrency integration. There is no timeout sweeper. If a caller omits release, the count stays elevated until process restart because only the owner of the request knows when it ended. Nil Engine receivers are unsupported.

### Test dependencies

`hooks_test.go` starts two calls for one key, releases one closure twice, and verifies the second call remains counted until its own release. Run `go test ./internal/hooks -count=1`; no external dependency is involved.

## Source responsibilities and entry points

- `hooks.go` is the entire production package. It defines `Engine`, its mutex and count map, one-time trace logging, `New`, and `Begin` with empty-ID handling, synchronized increment/decrement, per-call `sync.Once`, and zero-entry deletion.
- `hooks_test.go` verifies that release is idempotent without disturbing concurrent work for the same key.
