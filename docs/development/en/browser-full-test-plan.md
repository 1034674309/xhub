# Full browser test plan

Date: 2026-10-11 (Asia/Shanghai). Design baseline: Git `082da5e`. This is an execution plan, not a passing test report. Revision: visible, persistent-role, one-task-at-a-time acceptance with a global stop-on-error gate. The [Chinese edition](../zh-CN/browser-full-test-plan.md) provides the corresponding functional matrix.

## Mandatory execution contract: do one task, test it, accept it, then continue

This contract takes precedence over older instructions to continue independent tasks after a failure. Execute exactly one business task at a time across the entire acceptance run. Do not queue subsequent browser actions, switch to another role's business task, or run unrelated acceptance tasks in parallel before the current task is accepted.

The mandatory loop is:

1. **Prepare:** print the task ID, role, session, prerequisites, intended UI actions, expected outcomes, verification steps and cleanup obligations. Capture the current resource/usage baseline. Unknown expected behavior must be resolved from the product contract before executing, not inferred from whatever the product returns.
2. **Execute:** perform that task in the visible browser, using the current role's existing session. Print every step before performing it. API fixture preparation or protocol-client actions are explicitly labeled and cannot substitute for the UI action under test.
3. **Test immediately:** run the task's browser assertions, refresh/readback, real backend contract and persistence checks, plus relevant data-plane, permission, usage and billing checks. Perform the task-specific unit/backend regression checks where applicable. Do not defer correctness to a final bulk regression.
4. **Accept:** compare actual outcomes against every declared assertion and save evidence for the same resource/call IDs. Mark the task ACCEPTED only when all mandatory checks and applicable cleanup checks pass. Log the reason and evidence; the actor performing acceptance must not accept solely on the action executor's success message.
5. **Continue:** select the next task only after the previous task's acceptance record has been saved. Keep the same role session until its planned role-switch boundary. Repeat until the frozen inventory is exhausted.

On the first unexpected result, timeout, missing mandatory evidence, assertion failure, unexpected skip, ambiguous state, session identity mismatch or cleanup failure, immediately mark the task FAILED or BLOCKED and the entire run STOPPED. All later tasks remain NOT EXECUTED, including otherwise independent tasks. Finish only safe evidence capture and cleanup/rollback of this run's resources. An expected negative test (for example a contract-correct 403) is not a failure when its rejection, absence of writes and other required assertions all pass.

Do not automatically retry a business mutation, silently re-login, weaken an assertion, accept both success and failure, or let a passing retry erase the first failure. Investigate read-only, report the first incorrect step, expected/actual result, affected resources, screenshots/trace and cleanup status. Product fixes require authorization within the current task scope; this plan does not itself authorize repairs.

Resume only after the cause is resolved and resumption is authorized. Revalidate prerequisites and session identity, rerun the entire failed business task including normal/boundary/negative/recovery checks, and obtain a new ACCEPTED record before advancing. If resources, prices, permissions or environment have changed, rerun affected previously accepted dependencies. Record any necessary reauthentication explicitly; never claim the original session remained continuous when it did not.

Acceptance is evidence-based confidence within the declared local environment, not a mathematical guarantee of correctness for every external system or untested input. A known unresolved mismatch cannot be accepted.

## Visible RPA-style sessions and live task output

Use a visible Chromium window and one separate browser context per real identity. Log in once at the beginning of a role's journey; retain its cookies, page and session across navigation, dialogs, refresh and successive tasks. Switch existing role contexts at declared coordination points rather than logging in at every step. Logout, expiry, disabled accounts and SSO login are explicit authentication tasks with their own acceptance gates.

Print the full role task queue first, then print each step and the task's acceptance result in real time. Persist the same events in the report. Give each task its own deadline, evidence folder and acceptance record; do not hide dozens of business tasks behind one final PASS. Example:

```text
[ROLE START] organization-A administrator | session org-A-admin
[TASK QUEUED ORG-03] Allocate a team budget and reject over-allocation
[TASK START ORG-03][STEP 1/6] Read organization and team baseline
[TASK STEP COMPLETE ORG-03][STEP 1/6] Baseline recorded
[TASK VERIFY ORG-03] UI readback + backend + persistence + actual request limits
[TASK ACCEPTED ORG-03] All mandatory assertions passed | evidence task-ORG-03
[TASK START ORG-04] Reassign a member and verify old access is revoked
[TASK FAILED ORG-04] Expected 403; received 200 | evidence task-ORG-04
[RUN STOPPED] ORG-04 is not accepted; ORG-05 and all later tasks NOT EXECUTED
```

Display pacing may make operations easier to watch, but fixed sleeps are never evidence of success. Preserve screenshots safely and redact secrets. If visible browser control is unavailable, mark the visible acceptance run BLOCKED; headless regression cannot be presented as fulfilling the visible-session requirement.

## Scope and evidence

Validate complete user journeys through the real console, gateway, isolated PostgreSQL database and local protocol suppliers. Opening a page or receiving HTTP 200 alone does not establish success. Configuration changes must be performed in the browser, read back after refresh and verified through their actual business effect. Associate calls, logs, usage and billing with the same resource or call ID.

Freeze inventories of pages, navigation, dialogs, roles, memberships, supported actions and corresponding backend/data-plane effects. Map each case to that inventory. The initial inventory contains 45 page route templates, including dynamic documentation and compatibility redirects. Expand `/docs/[[...slug]]` into actual document URLs; conditional `notFound()` does not make this a retired route. `/router-settings` and `/models-and-endpoints/catalog` need redirect assertions.

Initial Playwright discovery reported 151 cases in 48 files, with 49 spec files in the directory. Reinventory before each run. Live cases are not registered by default, and unbound-endpoint cases require a separate fixture. Discovery is not execution or proof of complete coverage. Existing isolated tests are supporting regression, not proof of a continuous-role journey.

## Environment and safety

Use production console builds, Chromium, the real gateway, isolated database schemas and local suppliers. Never write into the shared development environment, stop user-owned services, close unrelated tabs or clear shared Redis. Use run-specific resources and an independent Redis instance where needed. Record resource IDs, ownership, original configuration and cleanup responsibility. Keep browser/acceptance locks intact and archive shared artifacts before starting another browser stage.

Do not run `make testdata`: it clears shared data. Do not run `make e2e`, `make e2e-all`, `--live` or paid supplier calls without separate authorization, declared credentials and a spending limit. Missing dependencies are blocked, not passed.

## User perspectives

Cover anonymous visitors, platform administrators, platform read-only administrators, organization A administrators, team A1 administrators, ordinary members, users without a team, ordinary read-only users, organization B comparison users and disabled/expired identities.

Platform roles, organization membership and team membership are separate authorization dimensions. Verify real backend grants rather than changing frontend labels. Use separate browser contexts for identities. Test both authorized access and forbidden access, including same-organization cross-team access, cross-organization access, lists, details, search, exports and logs. Usage scope follows each endpoint's contract rather than an assumed organization-wide view.

## Task lines

| ID | Complete journey | Essential outcome |
| --- | --- | --- |
| T01 | Documentation → login validation → sign in → refresh → logout → protected access | Real session behavior and localized errors |
| T02 | Organization → teams → users → projects → membership changes → role logins | Correct ownership, changed access and preserved historical attribution |
| T03 | Role-scoped lists → search → details → forbidden access and exports | Positive access, contract-correct denial, no leaks or partial writes |
| T04 | Supplier → discovery → deployment → connection test → edit → disable → restore → delete | Persisted protocols/prices, hidden secrets and actual availability |
| T05 | Personal key → call → regeneration → block → restore → delete | Correct ownership, old secrets invalidated, safe historical logs |
| T06 | My models → English examples → Playground → logs | Instructions match real requests, usage and bills |
| T07 | JSON/SSE chat → continuation/edit → cancel → clear → refresh | Complete responses, correct continuation and no delayed-response contamination |
| T08 | Image generation/edit and video tasks → polling → completion/failure → logs | Correct native paths, task lifecycle and no duplicate polling bills/logs |
| T09 | Templates → groups/weights/fallbacks → bindings → preview → calls → deployment deletion | Actual selection matches policy, no authorization bypass or stale references |
| T10 | Guardrail draft → debug → save/edit → enable → calls → monitor → delete | Blocking avoids upstream calls, redaction reaches upstream, invalid rules cannot save |
| T11 | Budget/RPM/TPM allocation → boundary → rejection → recovery | Hierarchical limits, no over-allocation or incorrect rejection billing |
| T12 | Successful/failed/guarded/media requests → filtering → pagination → details → CSV | Logs, snapshots, usage, bills and ownership reconcile |
| T13 | Cache settings → miss → hit → isolated flush → miss | Actual upstream call count and billing match cache behavior |
| T14 | Directory → SCIM enable/sync/mapping → SSO settings → archive settings | Correct memberships, one-time secrets, disabled rejection and queue behavior |
| T15 | Prompts/tags/budgets/policies/vector stores/search/transform/theme | Verify actual support and persisted effects for each exposed action |
| T16 | Website → help → search → API guide → model instructions → languages | Real documents, links, anchors, images, highlighting and useful examples |
| T17 | Navigation/tabs/dialogs → deep links → refresh → history → old entries | Every route has an explicit outcome, no blank denial or unexpected runtime errors |

Use two organizations, teams A1/A2/B1 and distinct users/projects/keys/logs to expose isolation failures. Each task declares fixtures and dependencies. Execute preflight, identity/resource setup, permissions, calls, policies/limits, reconciliation, settings/documentation/navigation and final cleanup, strictly one accepted task at a time. Restore disruptive changes immediately or isolate them. Independent tasks do not continue after failure.

## Expanded role and specialist journeys

The minimum design inventory is 12 journeys and 108 task nodes. Each semicolon-separated item below is one separately identified task, expanded into concrete UI steps and a verification/acceptance checklist before implementation. These are planned nodes, not implemented or passed tests. Map every node back to T01–T17 and report designed, implemented, executed and accepted coverage separately.

| Journey | Sequential task nodes |
| --- | --- |
| ADM-01…12: platform administrator | Authenticate and verify identity; create organizations A/B; create A1/A2/B1 teams and projects; establish real organization/team memberships; connect local suppliers and discover models; configure protocols/prices/access; deploy and test connections; configure routing groups/weights/fallback; configure and exercise guardrails; allocate budgets/RPM/TPM; reconcile platform logs/bills/audit; disable/restore/delete deployments and dependency references |
| ORG-01…10: organization A administrator | Verify organization-only identity; inspect owned teams/members/projects; create/edit owned teams; invite/reassign members; allocate authorized budgets/rates; reject over-allocation/invalid limits; inspect contract-allowed A1/A2 statistics; reject B details/search/export; reject platform supplier/model/global-setting writes; revoke access and clean resources |
| TEAM-01…10: team A1 administrator | Verify team identity; manage owned members; create/edit projects; allocate member/project budgets; allocate supported RPM/TPM; use authorized models/keys; inspect team calls/cost/export; reject A2/B1 resources and foreign ownership changes; reject privilege/limit escalation; remove a member and verify access/history |
| MEMBER-01…10: ordinary member | Verify personal scope; inspect models/English examples; create a personal key in UI; perform JSON/SSE and continuation calls; exercise image/video success/failure; inspect own logs/cost; rename/regenerate key; disable/reject/restore; hit/recover budget/RPM/TPM; delete key and verify old-secret rejection with retained historical logs |
| PERSONAL-01…08: user without a team | Verify no fictitious team; reject calls without a billing source; authorized actor configures personal budget; create key and call; reconcile personal user/project/key attribution; exhaust/restore budget; exercise personal RPM/TPM; delete key/account under historical-data contract |
| READONLY-01…06: two read-only identities | Authenticate separate platform/personal read-only contexts; verify allowed read scope; isolate list/detail/search/export; hide/disable forbidden UI writes; reject direct writes without partial persistence; verify billable-action permissions according to the actual role contract |
| LIMIT-01…12: quota executor | Validate fields; allocate hierarchical budgets; reject parent over-allocation; test exact monetary boundaries; test concurrent reservations; release failed/canceled reservations; hit each supported RPM scope; recover after RPM window; hit each TPM scope; reconcile estimated/actual tokens; test combined constraints; verify configuration/restart/reset semantics and audit |
| BILL-01…10: finance/audit perspective | Fixed-price request; reconcile SSE/non-SSE; attribute continuation; guardrail costs; failure/timeout/cancel costs; cache billing; retry/fallback deduplication; media polling settlement; role-scoped filter/pagination/CSV; price changes/timezones/aggregates/queue replay |
| SSO-01…10: enterprise administrator and login user | Create organization-bound connection in UI; validate fields/secrets; hide disabled entry; enable localized entry; complete local IdP redirect/callback login; map least-privilege identity; repeat login idempotently; reject invalid/expired/replayed state/nonce/callback; verify disable/logout/account-session contract; delete connection/organization and dependencies |
| SCIM-01…10: directory administrator | Create directory/one-time credentials; enable synchronization; provision user visible in UI; update user; provision groups and map roles/teams; test duplicate/filter/pagination/idempotency; move group/revoke access/preserve history; deactivate user and verify sessions/keys; reject disabled connection/invalid token/cross-tenant access; rotate token/delete resources |
| OPS-01…06: operations perspective | Cache toggle/isolated flush; verify supported configuration effects; archive queue/upload/readback/disabled backlog; recover queue/database/Redis faults; inspect diagnostics/audit secrecy; verify cleanup and service shutdown |
| VISITOR-01…04: anonymous/documentation user | Help/search/deep links; invalid login/protected access; languages/mobile navigation; execute an English documented request against the real gateway |

Finance, directory and operations are user perspectives, not invented RBAC roles: state the actual identity used. If a role or feature is unsupported, declare that gap and verify the supported denial behavior; do not grant platform privileges to make an organization/team task succeed. A required unsupported capability blocks the corresponding acceptance objective.

## Budget, RPM and TPM acceptance matrix

First establish which organization, team, project, user/member, key and global constraints actually exist and how they inherit. Personal billing is a separate source, not a fabricated team. Lower one constraint at a time while all others have sufficient capacity; then test two binding constraints and documented rejection precedence.

| Dimension | Mandatory checks and acceptance evidence |
| --- | --- |
| Input/allocation | Missing/null/zero/negative/fractional/large/invalid inputs; parent over-allocation; parent decrease; cancellation and repeated submission. Interpret zero/unlimited by contract, reject invalid writes atomically, refresh and read back actual values. |
| Money | Balance above/equal/below deterministic request cost; exhausted rejection; authorized increase and recovery. Apply reservation/settlement rules, prove rejected calls do not reach upstream or incur a successful-call charge. |
| Concurrency | Simultaneous calls competing for small remaining capacity; successes/failures/cancel/timeouts. Prove reservations do not overspend, releases occur, settlement is not duplicated. |
| RPM | Dedicated subject with L=3: first three counted requests allowed, fourth rejected within the specified window; independent subject unaffected; window expiry restores capacity. Verify actual counting semantics and rejection category. |
| TPM | Deterministic token usage below/equal/above threshold; oversized request, streaming, concurrent estimates and actual-use reconciliation. Verify release and distinguish token rejection from money rejection. |
| Interactions | Budget-only/RPM-only/TPM-only/combined exhaustion; increasing one leaves other constraints enforced; key rotation cannot bypass user/team limits. |
| Lifecycle | Limit edits, membership transfer/removal, disablement, supported cycle reset and restart. New calls use current rules, history is immutable, Redis and persisted state reconcile. |

Use fixed local supplier usage and frozen prices with an explicit rounding rule. Configure limits through the UI, trigger calls, observe denial, recover and accept before advancing. Supporting backend checks cover precise clock/concurrency cases; do not claim RPA clicks alone prove all stress boundaries. Isolate Redis and subjects.

## Billing and metering acceptance

Freeze currency, rates, rounding, cache/media pricing, failed-call metering, retry charging, ownership and idempotency rules before testing. For ordinary token billing, compute expected cost as input tokens × input rate + output tokens × output rate; apply the actual distinct cache/media contract where applicable.

Correlate each UI call with request/call ID, received upstream body/count, persisted usage and price snapshot, user/project/team/organization/key attribution, detail/list/aggregate and CSV. Compare run-specific deltas, not unrelated database totals. Every permitted role must see the same relevant facts in its authorized scope; forbidden scopes must remain inaccessible.

Accept success, SSE, continuation, failures, timeouts, interruption/cancel, guardrail block/redaction, cache, retry/fallback, media completion/failure and duplicate polling separately. Verify pagination has neither missing nor duplicate rows, CSV equals the filtered row set, date/timezone boundaries follow the contract, and new prices do not rewrite historical snapshots. Inject isolated Redis/database commit/ack failures and verify recovery without double billing. A mismatch stops the run immediately.

## SSO and SCIM acceptance

SSO and SCIM have separate fixtures and task records, but neither proceeds after a global acceptance failure. SSO must use a local deterministic IdP with real visible authorization navigation and callback establishing a least-privilege session. Verify supported issuer/audience/signature/state/nonce validation, invalid/expired/replayed callback rejection, idempotent account association, enable/disable behavior and explicit existing-session semantics. Do not infer SAML or any other unsupported protocol from a settings form. Configuration-only verification does not accept the full login task.

SCIM writes are explicitly labeled local directory-client actions, not UI operations. The browser creates/enables/maps the connection and verifies synchronized users/groups and their permissions. Test supported Users/Groups operations, PUT/PATCH, filter/pagination, duplicate external IDs/conflicts, deactivation, credential rotation and cross-connection isolation. Unsupported operations must return the documented error, not partial updates.

Test SCIM-created identity → SSO login association, repeated login/sync without duplicate accounts, stable tenant-scoped matching rather than mutable display names, group removal/deactivation with session/key invalidation according to the stated contract, and immutable historical usage attribution. Delete users/groups/mappings/connections/organizations in supported order and verify reverse-order deletion is rejected clearly where appropriate, not an unexplained 500. Business cleanup failure stops acceptance even if final schema cleanup succeeds. Missing local IdP/directory fixtures block full enterprise acceptance; backend-only success cannot replace browser evidence.

## Per-task acceptance record and run ledger

Every record includes task/role/session IDs, dependencies, resource baseline and ownership, exact UI/protocol steps, expected/actual outcomes, language/viewport, UI refresh assertions, backend status/body contract, persistence checks, data-plane/log/usage/billing evidence, commands/exit codes, cleanup or explicitly retained dependency resources, and the acceptance decision. Non-applicable checks require a reason recorded before acceptance; silently omitting a required check is BLOCKED.

Task states are QUEUED → RUNNING → VERIFYING → ACCEPTED, or FAILED/BLOCKED → RUN STOPPED. Acceptance is not the same as an action being completed. Counts distinguish steps, tasks, automated tests and role-matrix cells. Preserve first failure and separate reacceptance history. The ledger records the last accepted task, first failed task, all not-executed tasks and the exact authorized resumption point.

Resources retained for a later task require IDs, ownership, verified state and a cleanup obligation before current acceptance. Final cleanup is itself a gated task. Cleanup is the only allowed mutation after stopping, and is limited to safe rollback of this run's resources; evidence capture must not cause further business calls.

## Cross-cutting checks

Every task includes normal, boundary, failure and recovery cases; write flows include cancellation without persistence, refresh readback and cleanup. Verify Chinese and English labels, errors, empty states, accessible names and dynamic parameters. Examples and default prompts stay English; original user input is not translated. Cover keyboard/focus, validation, pagination boundaries, repeated submissions and browser history. Run desktop journeys and key narrow-screen journeys.

Use isolated supplier faults for timeouts, rejection, malformed bodies, interrupted streams and failed asynchronous jobs. Exempt only the intended failing request; unrelated errors still fail. Await observable conditions with deadlines, not fixed sleeps. Do not infer weighted-routing correctness from a few random samples.

Reports, screenshots and logs must not expose passwords, tokens, API keys or enterprise secrets. Delete created resources even on failure/interruption; verify deletion through UI/backend/call rejection and separately verify schema removal, process shutdown and configuration restoration.

Computer Use exploration follows observe → act → inspect → assert. Evidence-based locator correction is allowed only before a business action is submitted; once an unexpected business result or mandatory verification failure is observed, stop under the global gate. It complements, but does not replace, reproducible repository E2E. Count these modes separately.

## Execution commands

The commands below are supporting regression entries, not an implementation of the persistent-session acceptance runner. Record exact commands, exit codes and logs actually executed. Replace `<run-id>` and use absolute report directories. Do not filter the full baseline because that disables the existing full-coverage gate. Use `--headed` for visible runs and `--max-failures=1` for the first-failure gate; this flag alone does not provide per-task acceptance or eliminate per-test logins. The role-task orchestrator must implement the contract above before claiming continuous-role acceptance. Do not run the whole supporting list after an earlier stage fails.

```bash
cd frontend
npx playwright test --list --reporter=line
cd ..
bash scripts/regression.sh -v
E2E_LIVE=0 E2E_FULL_COVERAGE=1 E2E_BROWSER_REPORT_DIR=/absolute/run-dir/baseline bash scripts/e2e.sh --headed --max-failures=1
E2E_LIVE=0 E2E_ENDPOINT_UNBOUND=1 E2E_BROWSER_REPORT_DIR=/absolute/run-dir/unbound bash scripts/e2e.sh model-endpoints.spec.ts --grep 'unbound model' --headed --max-failures=1
npm --prefix website test
npm --prefix website run build
npm --prefix website run audit:links
npm --prefix website run test:e2e
```

Redis cases require a run-specific instance and `XHUB_REGRESSION_REDIS_URL`. Inspect all skip reasons and run additional fixtures where required. When changing implementation, add relevant unit/component/integration, backend regression and E2E coverage with the project's Chinese function comments and i18n requirements. Record defects first; a test request is not unlimited permission to redesign the product.

`E2E_BROWSER_REPORT_DIR` automatically retains only results.json and junit.xml. Before the next run, separately archive frontend/playwright-report, frontend/test-results, frontend/.e2e-report, e2e/e2e-report.txt and sanitized gateway/upstream/cleanup logs. Do not archive secret-bearing c.yaml or present older artifacts as current evidence.

## Completion and external boundaries

Report passed, failed, skipped, blocked and unexecuted separately, retaining first failures even after successful retests. Count automated cases, task lines, pages and role-matrix entries separately against frozen inventories. Local full completion requires all mandatory cases and special fixtures to pass, no unexplained skips, negative authorization checks and successful cleanup.

Deliver changed files, actual commands/exit codes, counts, coverage mapping, reproducible failures, report paths and cleanup evidence under `.e2e/computer-use/<run-id>/`. Previous isolated-suite results are historical evidence, not results for this revised acceptance loop. This revision changes the plan only; the 108-node continuous-role runner is not yet implemented or executed. A stopped run is incomplete, never a full pass.

Local suppliers prove protocol and gateway behavior, not real model quality, external DNS/TLS/rate limits, supplier charges, media generation, enterprise interoperability or real Kodo writes/permissions. Full browser OIDC redirects and archive uploads require available fixtures or must remain blocked/uncovered. Backend protocol success cannot be counted as browser success. Real suppliers, enterprise tenants and Kodo acceptance remain separately authorized work.
