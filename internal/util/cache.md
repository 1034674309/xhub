# Process-local response cache

## Overview

The cache package owns a bounded response cache inside one gateway process. It stores up to 8,192 opaque response bodies in least-recently-used order. Cache eligibility, request identity, and success criteria belong to the data plane; this package only provides deterministic keys and isolated byte storage. It has no TTL, persistence, Redis synchronization, or cross-process invalidation.

## External interfaces

- `New() *DualCache` creates an initialized 8,192-entry LRU. The capacity is a compile-time constant. Construction panics only if that constant is invalid, which would be a programming defect rather than a runtime storage failure. Gateway startup creates the instance shared by its data-plane calls.
- `Key(parts ...string) string` returns a lowercase SHA-256 digest. It hashes each part in order and appends a zero-byte separator after each one, so different segmentations cannot collapse into the same concatenation. Callers supply all response-changing dimensions, including caller scope, operation, public model, request body, deployment configuration, and routing context.
- `(*DualCache).Get(key string) ([]byte, bool)` returns a newly allocated copy and true on a hit. A miss returns nil and false. Mutating the returned slice cannot alter the stored entry.
- `(*DualCache).Set(key string, value []byte)` copies the input before insertion. Later mutation of the caller's slice cannot affect the cache. Insertion updates recency and may silently evict the least-recently-used entry.
- `(*DualCache).Flush()` purges every entry in that instance. The gateway management flow uses it to invalidate local response data; it does not alter budgets, logs, request queues, or another process.

The package registers no HTTP route. internal/dataplane performs lookups and writes, while internal/gateway owns construction and administrative flushing.

## Core implementation

DualCache wraps the concurrency-safe generic LRU from hashicorp/golang-lru/v2. New fixes capacity at 8,192, making memory growth bounded by entry count. Key streams UTF-8 bytes directly into SHA-256; the explicit separator preserves part boundaries, and hexadecimal encoding yields a stable string suitable for the LRU map.

Set allocates an equal-length slice and copies before calling Add. Get first obtains the stored slice, then allocates and copies again. The cache therefore never transfers ownership of its internal byte slice in either direction. This prevents data races and accidental corruption when handlers reuse buffers, at the cost of one full copy per read and write. The LRU library serializes map and recency operations. A miss, replacement, or capacity eviction is expected behavior and produces no error. Nil receivers are not supported; empty keys and zero-length values are ordinary entries.

The data plane controls the surrounding sequence: extension approval precedes lookup, and only eligible completed responses are inserted. Streaming failures and protocol decisions are outside this package, so adding data directly through Set bypasses those policies by design.

### Test dependencies

`cache_test.go` checks Flush, zero-byte part separation, and both copy boundaries. Run `go test ./internal/util -count=1`; the suite is entirely in memory.

## Source responsibilities and entry points

- `cache.go` is the complete production implementation. It defines `cacheEntries`, `DualCache`, one-time trace logging, construction, SHA-256 key generation, copied lookup/insertion, and purge. Public entry points are `New`, `Key`, `Get`, `Set`, and `Flush`.
- `cache_test.go` is the package contract suite for invalidation, key framing, and slice ownership.
