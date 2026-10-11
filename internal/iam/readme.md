# Identity, authorization resources, quotas, and accounting

[简体中文](readme_cn.md)

## Overview

`internal/iam` is the PostgreSQL-backed identity and accounting domain used by authentication, the management gateway, inference admission, and usage reporting. It owns users, organizations, teams, memberships, projects, API keys, route-template bindings, audit records, immutable usage events, request bodies, daily aggregates, budget spend, and RPM/TPM assignments. The package exposes Go methods only; HTTP authentication and role checks live in the gateway before these methods are called.

The resource hierarchy is organization → team → user → key. Projects narrow team permissions but are not a separate parent in the quota tree. A person may belong to at most one team because billing and rate admission require one unambiguous ancestor path. All accounts use the same user model; every key belongs to a user, with no business key or special application account. Plaintext passwords and keys are never persisted: passwords use bcrypt and keys are stored by hash, with plaintext returned only when a key is created or rotated.

Usage is append-oriented and idempotent. A successful batch writes the immutable event, optional request/response log, daily aggregates, and spend on all applicable scopes in one transaction. The event stores ownership, provider/model, token counts, and the applied price snapshot from the time of the call; later membership or catalog changes do not rewrite history. Asynchronous jobs use a separate lifecycle that records one initial request and settles cost exactly once when the job completes.

## External interfaces

### Database lifecycle and errors

`Open(ctx, dsn) (*DB, error)` opens pgx through xorm, applies `schema.sql`, and aligns application, xorm, connection-session, and PostgreSQL time zones. `Migrate` reapplies the idempotent schema and `Close` releases the pool. Database failures propagate; common constraint outcomes are normalized to `ErrNotFound`, `ErrConflict`, or `ErrInvalid`. `RecordAudit`, `ListAudit`, and `QueryAudit` write or page audit records. Search is applied before count and pagination, and display enrichment prefers immutable names captured in the audit detail before batch-loading current names.

### Users, organizations, teams, projects, and keys

`Bootstrap`, `EnsureAdmin`, `Login`, `CreateUser`, `GetUser`, `UserByEmail`, `UpdateProfile`, `SetPassword`, `AdminUpdateUser`, and `DeleteUser` implement account lifecycle. Inputs are normalized and validated; passwords are hashed; operations that would remove the last platform administrator are rejected. `ListUsersPage(UserListQuery)` intersects search/status/role/team/organization filters with the caller-supplied authorization scope, uses a whitelist for sort fields, clamps limit to 100 by default and 500 maximum, and returns `(rows, total, error)`. SSO filtering currently returns no rows because IAM stores no SSO identifier.

Organization and team entry points include `CreateOrg(..., rates ...RateLimits)`, `UpdateOrg`, `CreateTeam`, `UpdateTeamProfile`, `AdminUpdateTeam(..., rates ...RatePatch)`, `MoveTeam`, and delete/list/get methods. Membership methods add, update, or remove organization administrators and team members while keeping at least one team administrator and rejecting a second team for the same person. Project methods enforce that project models and budget stay within the parent team. Mutations write audit records in their transaction; foreign-key or allocation failures roll the whole mutation back.

`CreateKey` and `RotateKey` return `(key, plaintext, error)` and expose the secret once. `KeyByHash` is the authentication lookup; `ActiveKey`, `OwnedBy`, `TouchKey`, status/delete/reset methods, and filtered listing support gateway policy. `AllowedModelsForTeam` and `AllowedModelsForKey` intersect team, project, and key model sets; nil means no restriction at that level. `ParseExpiry` accepts the supported relative duration syntax and rejects malformed values. `ResetKeySpend` changes the live counter without deleting historical events.

### Quota, rate, template, and usage APIs

`QuotaPath(ctx, kind, id, hot)` returns `(teamID, organizationID, exhaustedScope, error)`. It reads the complete hierarchy, adds unflushed Redis spend through `hot`, reserves unused fixed child allocations, and reports the first exhausted scope on the target path. Missing parents, ambiguous memberships, and invalid allocations return errors. This is a pre-call admission check; a supplier's final charge can still exceed the last available balance.

`ValidateRates` rejects negative or PostgreSQL-`INT`-overflowing RPM/TPM values. `RatePlan(ctx, kind, id)` returns a `live.RatePlan` containing the target path plus siblings and descendants whose fixed allocations reserve ancestor capacity. Missing nodes, ambiguous parents, and cycles fail. IAM is responsible for producing the valid acyclic indexes and path consumed directly by `live.CheckRatePlan` and `live.AdmitRatePlan`.

Route-template APIs create, update, list, read, and delete JSON bodies and bind organization/team/project scopes. `Settings` turns malformed stored JSON into an empty settings document so one bad row cannot stop request routing. `RouteTemplateUsage` and `RouteTemplateUsageCounts` identify bindings; gateway handlers check usage before deletion. An empty binding restores inheritance.

`RecordUsage(records)` validates the complete batch before writing, removes duplicate request IDs within the batch, and relies on the unique database request ID for cross-request/concurrent idempotency. It returns an error and rolls back every event, log, aggregate, and scope counter if any write fails. `RecordTaskUsage(record, initial, settle)` serializes one task with a PostgreSQL advisory transaction lock: initial creation counts one request with zero cost, polling updates status/body only, and the first completed settlement adds tokens and spend. Ownership and start time always come from the original event.

`UsageQuery` carries both authorization scope and filters. `ListUsageSessions`, `ListUsage`, `CountUsage`, `GetUsageEvent`, `GetRequestLog`, `DailyUsage`, and model/key/team rollups put the authorization predicate into SQL before ordering, count, and pagination. `AuditLogRead` records privileged request-log access. SQL, decoding, and context errors propagate; empty result sets are returned as empty slices, while single-record lookups return `ErrNotFound`.

### Actual HTTP registration and callers

This directory does not mount HTTP handlers. `internal/gateway/identity/mount.go` maps the APIs to `/user/*`, `/organization/*`, `/team/*`, `/project/*`, `/route_template/*`, and `GET /audit/logs`; login/bootstrap handlers in `internal/gateway/routes.go` also call IAM. `internal/gateway/keys/mount.go` registers key generation, service-key creation, list/info, update, block/unblock, delete, bulk, rotate/reset, aliases, and key health endpoints. `internal/gateway/usage/mount.go` registers spend logs and detail, session/global reports, daily activity, model/key/team/provider/tag rollups, cost calculation, and health endpoints. Gateway middleware supplies the actor and authorized scope and maps IAM errors to HTTP status codes. Inference authentication calls key/user/model checks, `QuotaPath`, and `RatePlan`; the live spend flusher calls `RecordUsage`.

## Core implementation

### Transactions, constraints, and time

`DB.tx` binds context, begins a transaction, rolls back on callback or commit failure, and commits only after every domain write succeeds. Resource mutations lock the affected user, organization, or team where concurrent allocation changes matter. `schema.sql` supplies unique request IDs, membership and binding foreign keys, checks, indexes, aggregate tables, and audit storage. The connection string receives a named session zone and the server receives the matching POSIX zone so application-created timestamps and `DEFAULT now()` round-trip to the same instant, including UTC+8 deployments.

### Budget and rate allocation

`quotaTree` loads organizations, teams, users, memberships, and keys and links them into one forest. Fixed budget children reserve `max(limit-spend, 0)`; shared children pass the sum of fixed descendant reservations upward. `validateQuota` checks only the changed node and its ancestors, which permits an already-invalid legacy tree to be repaired incrementally without blocking unrelated branches. At admission time `QuotaPath` applies hot Redis spend to both the path and reservation calculations and credits the target child's own reservation so it is not blocked by capacity allocated to itself.

RPM and TPM use the same topology. `rateAllocated` sums fixed children and passes fixed descendants through shared nodes; zero is a real limit and nil means shared/unset. `buildRatePlan` walks the target to its root with cycle and parent validation, then includes the complete rooted component so sibling reservations remain visible. It sorts node IDs for stable indexes and emits the target path from narrowest to broadest. Configuration updates validate nonnegative values and parent allocation inside the same transaction.

### Identity invariants and audit snapshots

Email normalization is case-insensitive, passwords use bcrypt, and role conversion keeps storage roles separate from console compatibility names. Membership writes require active users, prevent multiple team ownership paths, preserve the last team/platform administrator, and revoke affected personal keys when ownership is removed. Model and budget narrowing computes intersections/ceilings across team, project, and key. Audit writes capture actor/object identity where available; reads never widen authorization to enrich a name, and deleted resources can therefore remain unnamed.

### Usage accounting and failure boundaries

`RecordUsage` first rejects missing IDs, negative/non-finite prices, invalid token counts, and malformed timestamps before opening the transaction. `insertEvent` determines whether a request is new; only new events create logs, increment daily request counts, and add spend to organization, team, project, user, and key. All aggregates use the event's captured ownership and day. A database error leaves no partial counters. Redis queue acknowledgement happens only after this transaction commits, while database request-ID uniqueness makes replay after a crash safe.

Task accounting locks on a hash of request ID even before a row exists. Repeated initial calls are no-ops; polling cannot add request count or spend; failed tasks cannot return to an executing state; settlement requires `completed` and a matching key/user/model. A zero-cost completed result is still marked settled. Usage readers construct authorized SQL first, then add caller filters, deterministic ordering, totals, and page limits; local-day reports shift timestamps by the requested minute offset.

### Dependencies and verification

The package depends on PostgreSQL/pgx/xorm, bcrypt, `internal/live` for rate-plan types, and `internal/util`. Unit and regression tests cover time-zone setup, key expiry and personal keys, quota/rate allocation, user pagination, route templates, audit filtering/snapshots, usage pagination/idempotency, and task lifecycle. Database tests need an available PostgreSQL test DSN and may skip when it is absent; Redis behavior is tested in `internal/live`, and external identity providers and model suppliers are outside this package.

## Source responsibilities and entry points

- [`audit_display.go`](audit_display.go) implements private audit text extraction, pre-pagination search predicates, and post-authorization batched display-name enrichment used by `QueryAudit`.
- [`db.go`](db.go) defines `DB`, normalized domain errors, `Open`/`Migrate`/`Close`, transaction/session and time-zone helpers, `Actor`/`Audit`/`AuditFilter`, audit writes, and audit list/query entry points.
- [`keys.go`](keys.go) defines key records and inputs, secret generation/hash/rotation, lifecycle and filters, model intersections, budget ceilings, expiry parsing, team-key revocation, and spend reset. Authentication enters through `KeyByHash`; management enters through create/update/list/status/delete methods.
- [`models.go`](models.go) defines persisted user, organization, team, membership, project, audit, and bootstrap records, status/role constants, table mappings, active/admin predicates, and UUID generation.
- [`quota.go`](quota.go) builds and validates the private quota tree, computes fixed-child reservations and rate allocation, enforces one-team ownership, creates service-key business owners, and exposes `QuotaPath`, `ErrQuotaAllocation`, and `ErrMultipleTeams`.
- [`rates.go`](rates.go) defines `RateLimits`, patch semantics that distinguish omitted from explicit clear, numeric validation, `RatePlan`, and the private stable rooted-plan builder consumed by `internal/live`.
- [`route_template.go`](route_template.go) defines template ownership/storage, JSON settings decoding, CRUD, binding validation, scope-table selection, and usage listing/counting.
- [`task_usage.go`](task_usage.go) implements the advisory-lock task lifecycle and exactly-once completed settlement through `RecordTaskUsage`.
- [`teams.go`](teams.go) implements organization/team CRUD, visibility and administrator queries, membership invariants, project CRUD, hierarchy locks, model/budget narrowing, rate patches, and transactional audit writes.
- [`usage.go`](usage.go) defines events, request logs, daily rows, and `UsageRecord`; `RecordUsage` and private insert/log/daily/scope helpers implement atomic idempotent accounting.
- [`usage_read.go`](usage_read.go) defines authorized `UsageQuery`, sessions and aggregate result types, SQL scope/filter/order builders, event/log lookup, daily reports, rollups, and privileged-read auditing.
- [`user_pagination.go`](user_pagination.go) defines `UserListQuery`, authorization-aware directory SQL, sort whitelist, limit/offset normalization, and `ListUsersPage`.
- [`users.go`](users.go) implements bootstrap, login and bcrypt handling, role conversion, account CRUD, scoped/simple lists, locking, last-admin protection, and profile/password/admin updates.
- [`schema.sql`](schema.sql) is the idempotent PostgreSQL schema for all IAM resources, constraints, indexes, usage aggregates, request logs, audit records, route-template bindings, and bootstrap state; `Open` and `Migrate` execute it.
