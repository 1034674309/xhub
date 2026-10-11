# Framework PostgreSQL store

[简体中文](readme_cn.md)

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
