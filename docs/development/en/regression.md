# Backend regression

## Run

Use the real gateway, isolated PostgreSQL schemas and a local protocol upstream. Never use production records as fixtures.

The complete deterministic suite also requires `redis-server` on the test host's `PATH` (install the `redis-server` package on Ubuntu/Debian).
Spend-queue recovery, concurrent session affinity and agent conversation cases start their own Redis processes on random ports and stop them during cleanup.
They do not use or flush an existing Redis instance. Starting the Compose Redis service alone does not provide the required host binary;
CI explicitly installs it and checks its version before testing.

~~~bash
make regression
bash scripts/regression.sh -v Pricing
bash scripts/regression.sh -v Fallback
~~~

GitHub Actions saves the complete verbose output to `.e2e/release/regression.log`
and uploads it in the `release-test-results` artifact, including on failure. The
step prints a short success message or the final 16 KiB on failure, preserving
the regression exit status. Local runs print a short result; `-v` also shows test
start and pass/fail/skip status. Both modes save complete diagnostics to a unique
`.e2e/runs/regression-*/regression.log`; CI uploads these files too.

## Assert observable results

| Scenario | Assertions |
| --- | --- |
| Configuration save | HTTP result, reread and actual execution |
| Authorization | Success, cross-organization/expired identity rejection, list scope |
| Fallback | Actual upstream order, attempts and final model |
| Billing | Usage, snapshot, five scope totals, deduplication |
| Failure | Error response, no partial writes or success cache |

[Cases](../../../cmd/regression) · [Isolation harness](../../../cmd/regression/testsupport/postgres.go)

Clean up with t.Cleanup. Database skips do not establish a pass. Real-provider limits and billing require separate [paid verification](regression-live.md).
