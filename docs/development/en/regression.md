# Backend regression

## Run

Use the real gateway, isolated PostgreSQL schemas and a local protocol upstream. Never use production records as fixtures.

~~~bash
make regression
bash scripts/regression.sh -v Pricing
bash scripts/regression.sh -v Fallback
~~~

GitHub Actions saves the complete verbose output to `.e2e/release/regression.log`
and uploads it in the `release-test-results` artifact, including on failure. The
step prints a short success message or the final 16 KiB on failure, preserving
the regression exit status. Local `-v` runs still print the complete log.

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
