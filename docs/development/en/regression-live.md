# Live-provider verification (paid)

This runs real model requests. Run only with provider-call authorization, valid credentials and an approved charge budget.

~~~bash
make regression-live
~~~

scripts/with-live-vendors.py selects providers from config_provider.yaml and resolves keys through key_env or explicitly configured credential sources. Never put keys in YAML or logs.

| Prerequisite | Requirement |
| --- | --- |
| PostgreSQL | Isolated schemas and cleanup |
| Redis | Dedicated to this runner |
| Model | Account-authorized, with reconcilable rates |
| Evidence | Call ID, provider, deployment, usage, snapshot and amount |

Missing-credential SKIPs are not passes. Real providers cannot reliably produce exact 429/500 sequences; deterministic regression verifies retry counts.

[Runner](../../../scripts/with-live-vendors.py) · [Multi-provider](multi-supplier-regression.md) · [Free verification](regression.md)
