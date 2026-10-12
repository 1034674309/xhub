# Framework PostgreSQL store

[简体中文](readme_cn.md)

## Integration from other modules

Integration here means other Go packages in this repository. Go internal import rules restrict imports from outside the repository; HTTP clients use mounted gateway endpoints.

Open framework storage at startup and share it through the host. Authorize before reading/writing namespaced records and close Engine at shutdown. IAM owns identity, memberships, and accounts.

| Entry point or resource | Usage and result |
| --- | --- |
| Open(dsn) | Connect to PostgreSQL and synchronize framework tables |
| GetKV(kind, id) | Read a JSON record; missing records return sql.ErrNoRows |
| PutKV(kind, id, json) | Write a document and invalidate caches |

### Integration demo

This is a Go integration helper for a host. It compiles as a separate package inside the repository; callers must supply the real dependencies described below before execution.

```go
package demo

import "github.com/sunqirui1987/xhub/internal/store"

// ReadDemoRecord 演示已授权文档读取；传入专用数据库和命名空间，返回记录或错误。
// Open 会同步表结构；不修改业务记录，结束关闭 Engine。
func ReadDemoRecord(dsn, kind, id string) (map[string]any, error) {
	st, err := store.Open(dsn)
	if err != nil {
		return nil, err
	}
	defer st.Engine.Close()
	return st.GetKV(kind, id)
}
```

This helper compiles and requires dedicated PostgreSQL. Missing records return sql.ErrNoRows. Open can write table definitions.

### Errors, state, and side effects

GetKV bypasses caches for fresh security-sensitive values. Store does not authorize tenants. Do not overwrite user records with demo namespaces.

## Overview

`internal/store` persists gateway framework records that are separate from IAM: generic JSON documents, dashboard-managed model deployments, and namespaced configuration. It accepts only `postgres://` or `postgresql://`; empty, SQLite, file, and unknown URLs fail at startup. Identity, keys, memberships, audits, and usage belong to `internal/iam`.

The store wraps xorm with a 10,000-entry in-memory LRU. Writes clear both schema-qualified and unqualified cache entries. Security-sensitive `GetKV` reads bypass cache so another process's session or credential revocation is visible immediately. SQL logging is always suppressed because statements can contain hashes and credentials.

## External interfaces

### Opening and generic documents

- `Open(databaseURL) (*Store, error)` validates PostgreSQL, optionally creates a safe `search_path` schema, pings, and synchronizes `kv`, `proxy_models`, and `proxy_config`. It returns connection/schema/sync errors and closes partial engines. Gateway startup is the main caller.
- `PutKV(kind, id, body)` updates or inserts raw JSON text and invalidates cache. `GetKV` returns a decoded object, adds `id` when absent, preserves malformed bodies under `body`, and returns `sql.ErrNoRows` for nil/missing stores. `DeleteKV` returns `sql.ErrNoRows` when absent. `ListKV(kind)` returns all decoded rows; a nil store is an empty source. Sessions, credentials, invitations, guard/config documents, and UI settings use these APIs.

### Models and namespaced configuration

- `ProxyModel` is the public model record. `UpsertProxyModel`, `ListProxyModels`, and `DeleteProxyModel` JSON-encode/decode params and info and invalidate model cache; malformed stored JSON becomes an empty map. Database rows override same-name YAML deployments in gateway/model assembly.
- `PutConfig(namespace, key, value)` JSON-encodes and upserts a value. `ListConfig(namespace)` decodes all values and falls back to the raw string for malformed JSON; nil stores return an empty map so YAML remains authoritative. `DeleteConfig` removes a row. Gateway model-cost reload, routing defaults/groups, and preferences are direct callers.
- `SaveModelDirectory(entry, deletedID, directory)` atomically upserts/deletes a deployment and removes stale references from model defaults, routing groups, and IAM route templates. Either change argument may be empty; both empty performs repair only. Any marshal, malformed JSON, query, lock, or commit error rolls back. It clears caches after commit and does not modify the in-memory directory.

### Actual HTTP boundary

`internal/store` registers no routes. `gateway/models` exposes its effects through `POST /model/new`, `/model/update`, `/model/delete`, enable/disable and builtin routes, `PUT /model/default`, `PUT /model/fallback`, and `GET|POST|PUT|DELETE /routing/groups`. `gateway/prefs` uses config rows for `GET /config/list`, `POST /config/update`, `POST /config/field/update`, and `POST /config/field/delete`. Handlers perform authorization, validation, in-memory publication, and HTTP error mapping.

## Core implementation

### Engine and cache consistency

`ensureSchema` reads `search_path`, rejects empty/symbolic/>63-byte identifiers through `quoteIdent`, creates the schema over an admin connection, then `dsnWithSearchPath` installs PostgreSQL `options=-c search_path=...`. xorm maps private row structs and syncs tables without adding constraints. `quietLog` implements every xorm logger method while emitting no SQL. `bust` clears IDs and beans for both table-name forms after mutations; `GetKV.NoCache()` is the cross-process revocation exception.

### Atomic model-directory cleanup

`SaveModelDirectory` uses one xorm transaction. It writes the deployment, scans `model_defaults`, deletes defaults for zero/one remaining deployment, and cleans allocations when multiple remain. It parses every `routing_groups` row through router cleanup. If the IAM table exists, it locks all `route_templates` with PostgreSQL `FOR UPDATE` and rewrites their weights. Corrupt historical JSON is a hard failure so deployment deletion cannot commit while references remain partially cleaned.

### Dependencies and verification

The package depends on PostgreSQL/pgx, xorm, `internal/config`, `internal/router`, `internal/iam`, and `internal/util`. `model_weights_test.go` is a direct regression suite for transactional cleanup, damaged JSON rollback, defaults/groups/templates, and cache-visible results; broader gateway tests exercise KV, model, and preference callers. Database-backed cases require the configured test PostgreSQL schema and clean up fixtures transactionally or by isolated schema.

## Source responsibilities and entry points

### `store.go`

Defines `Store` and `Open`; enforces PostgreSQL-only configuration and delegates fully initialized engine creation.

### `engine.go`

Implements `quietLog`, `openEngine`, schema creation, search-path DSN rewriting, ping, LRU installation, and table synchronization. Error paths close opened resources.

### `pg.go`

Provides private `quoteIdent`, the strict PostgreSQL schema-identifier guard used before dynamic `CREATE SCHEMA`.

### `beans.go`

Defines the private xorm rows and table mappings for `kv`, `proxy_models`, and `proxy_config`, including composite primary-key order and persisted JSON/text columns.

### `crud.go`

Implements generic KV upsert/read/delete/list, composite IDs, missing-row normalization, JSON fallback, and cache invalidation.

### `models.go`

Defines public `ProxyModel` and the trace hook used when reading dashboard deployments; it deliberately keeps persisted row details private.

### `keys.go`

Implements model and namespaced-config CRUD, JSON conversion, nil-store fallback, timestamps, and cache invalidation.

### `model_weights.go`

Implements `SaveModelDirectory`: transactional deployment mutation and reference cleanup across store and optional IAM tables, explicit PostgreSQL row locking, rollback boundaries, and post-commit cache clearing.
