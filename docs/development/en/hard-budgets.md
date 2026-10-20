# API key hard budgets

When a key, user, team, organization, or project has a budget limit, XHub reserves an upper bound for each actual upstream attempt in a PostgreSQL transaction before sending it. Gateway instances share these durable reservations. Retries need separate reservations. Every applicable scope must have enough available budget.

## Output caps and available balance

A budgeted request without an output limit defaults to **4096 tokens**, reduced further by a smaller trusted deployment output limit. Valid explicit limits are retained. Conflicting limits or values above the deployment limit are rejected. The cap is written into the final upstream JSON after history restoration, guardrails, and protocol conversion, including native paths.

| Upstream protocol | Output field |
| --- | --- |
| Responses | `max_output_tokens` |
| Messages | `max_tokens` |
| Chat Completions | A trusted declaration of `max_completion_tokens`, or supported `max_tokens` |

When Chat supports both fields, XHub uses `max_completion_tokens`, which includes hidden reasoning. A deployment declaring only legacy `max_tokens` cannot run an explicit reasoning request under strict budgets. Messages `thinking.budget_tokens` must be below the total output cap. Reaching the cap can produce an incomplete or length-limited result; clients must handle that outcome.

Input is reserved against the **entire trusted input window**, without estimating tokens from character counts. Even a short prompt may therefore need a substantial available balance. A reservation is not recorded spend. Complete, reliable measured usage is charged at the frozen rates for that attempt, and the remaining reservation is released in the same transaction.

Admission headers expose `X-XHub-Max-Output-Tokens`, `X-XHub-Budget-Input-Bound`, `X-XHub-Budget-Reserved`, `X-XHub-Budget-Reservation-Id`, and `X-XHub-Budget-State`. Headers cannot track settlement after streaming starts; read the key details for the final state.

## Trusted deployment metadata

The input bound is read from `max_input_tokens` or `context_length`, checking deployment `model_info`, then `litellm_params`, then the matching model catalog. Chat also needs trusted `supported_parameters`. A model name or protocol compatibility label does not establish these capabilities.

The following is a **fictional configuration example**, not actual GPT, DeepSeek, Grok, or Claude specifications or pricing. Administrators must use verified provider limits and prices. Writing a smaller input bound does not make the provider enforce it. Keep requests blocked when reliable information is unavailable.

```json
{
  "model_name": "budgeted-text-example",
  "litellm_params": {
    "model": "provider-text-model",
    "input_cost_per_token": 0.000001,
    "output_cost_per_token": 0.000002,
    "cache_read_input_token_cost": 0.0000001
  },
  "model_info": {
    "pricing_source": "manual",
    "max_input_tokens": 32768,
    "max_output_tokens": 8192,
    "supported_parameters": ["max_completion_tokens"]
  }
}
```

Without cache writes, this example reserves at least `32768 × 0.000001 + 4096 × 0.000002 = 0.04096 USD` for a 4096-token output cap. This is not the final bill. Flat token prices are USD per token. Manual pricing must be complete; missing fields are not silently filled from the catalog. Cache reads, writes, and one-hour writes need their respective rates. Unknown tiers, conditions, or billing dimensions cannot be treated as free.

This version supports bounded text generation and client tool round trips. Multimodal requests, server tools/search, remote history references, unknown billing dimensions, and uncertified fields are explicitly rejected under strict budgets. Requests with no limit anywhere in their hierarchy keep the existing compatibility path. Removing limits is not evidence that hard budgets work.

## Errors and unresolved reservations

| HTTP / error code | Meaning and action |
| --- | --- |
| `400 invalid_request` | Invalid cap or request parameters; correct the request. |
| `400 unsupported_budget` | No provable cost bound; supply trusted metadata or use supported capabilities. |
| `429 budget_reservation_exceeded` | Available balance cannot cover the bound; reduce a valid output cap or adjust the budget through administration. |
| `503 quota_unavailable` | Quota storage is unavailable or old billing queues could not be drained; restore storage and reconcile the queues. |

Timeouts, cancellation, truncated streams, missing/conflicting/invalid usage, and usage above declared limits do not prove that the provider charged nothing. Submitted attempts retain an unresolved reservation. They are not automatically refunded on timeout, and unknown cost is not reported as reliably zero. If a provider violates its declared bound, XHub preserves the reservation and evidence; it cannot control what the provider charges its own account.

Authorized key details expose `budget_pending` and `budget_reservations`, with `reserved`, `submitted`, `settling`, and `uncertain` states. Attempt IDs and reasons support reconciliation with upstream bills. There is no one-click release or automatic TTL cleanup. For unresolved attempts, stop affected calls, preserve snapshots and upstream bills, and use an operational recovery procedure reviewed for those records. Do not bulk-delete reservations.

If usage is complete but the billing transaction fails, an independent `usage_pending` snapshot supports idempotent replay. The reservation is released only after the actual billing transaction commits. Whole-PostgreSQL unavailability remains a persistence limitation tracked by Issue #35; existing reservations stay conservative and unverified usage is not considered settled.

## Upgrade and configuration changes

1. Stop new affected calls, wait for in-flight attempts, and reconcile/drain old Redis billing entries and PostgreSQL `usage_pending`. Flushing Redis does not settle bills.
2. Upgrade every gateway node to the reservation implementation. Mixed deployment with old writers cannot provide this guarantee.
3. Verify model bounds, output-cap support, and complete rates before enabling limits. Changing an unlimited scope to a limited one also requires draining in-flight calls; earlier unlimited attempts cannot acquire a cost bound retroactively.
4. Test a dedicated limited key for default caps, concurrent rejection, measured settlement, and key details before restoring traffic.

Changes between existing finite limits must cover both settled spend and reservations. For organizations with fixed budget periods, old-period attempts remain available for reconciliation but do not occupy the organization's new period. Other scopes, including users and keys, retain their own budget rules.
