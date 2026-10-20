# Manual reconciliation of uncertain charges

A submitted request with no reliable final charge retains its `uncertain` reservation. Timeouts, HTTP 400 responses, budget period resets, and process restarts do not establish that an upstream call was free.

Platform administrator sessions use `GET /budget/reservations` and `POST /budget/reservations/{reservation_id}/reconcile`. Ordinary users, organization/team administrators, and virtual keys cannot use these management operations.

The POST body contains:

- `outcome`: `no_charge` or `charged`.
- `verified_cost_usd`: the actual cost of this **attempt**, not the cumulative cost of the call. It must be zero for `no_charge` and positive for `charged`.
- `evidence`: a reviewed basis of 1–2000 characters. Do not include credentials or user request bodies.
- `expected_fingerprint`: the reservation fingerprint returned by the list endpoint.

The list accepts `key_id`, `state`, `page`, and `page_size` (maximum100). A missing ledger cost is `null`, not proof of no charge. `can_reconcile=false` and `blocked_reason` identify incomplete evidence: a non-uncertain reservation, a missing/nonterminal event, identity mismatch, another active attempt for the same call, or pending PostgreSQL usage. Recover queued usage first; never delete pending evidence to enable reconciliation.

One transaction appends immutable evidence to `budget_reconciliations`, adds the verified attempt cost to the call, daily totals, monitoring costs and five billing scopes, records a `budget.reconcile` audit entry, and removes that attempt's hold. Request counts, tokens and performance samples do not increase. Original upstream pricing remains unchanged; the original hold and pre-correction event are preserved separately. Verified costs may exceed the reservation, since a confirmed expense must not be truncated. This may exhaust the remaining balance and block subsequent calls.

An identical retry returns `already_resolved=true`. A changed conclusion, stale fingerprint, or incomplete evidence returns409 `budget_reconciliation_conflict`; invalid input returns400 `budget_reconciliation_invalid`. There is no automatic bulk refund or operation to overwrite an existing resolution.

Original timestamps and frozen identities govern accounting. Charges from a previous organization period do not consume its current period. Revoked keys or deleted users retain historical attribution. Different attempts may use different suppliers; new reservations freeze their supplier and upstream model. A missing historical supplier stays unknown rather than inheriting the final attempt's supplier.

Redis, PostgreSQL pending usage, and filesystem queues ultimately replay through `RecordUsage`. A manually resolved attempt is acknowledged only when the late identity and charge agree. Conflicts return `ErrConflict` and retain the payload without charging again. A late settlement cannot remove a different hold against an already recorded call and silently lose its charge through event deduplication.

This version does not automatically resolve missing original events or crash remnants in `submitted/reserved` states. It does not fetch supplier invoices; administrators must obtain reliable evidence first. Deployment never automatically changes historical production reservations.
