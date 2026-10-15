# E2E regression

Use `make e2e-offline` for the real console, gateway and PostgreSQL suite with local protocol fixtures. Start the Compose PostgreSQL service and install Chromium in frontend first. Select actual scenario paths with:

```bash
bash scripts/e2e.sh scenarios/pages/docs.test.ts scenarios/docs
```

The equivalent Make entry is `make e2e-offline E2E_TESTS="scenarios/pages/docs.test.ts scenarios/docs"`. It forwards case paths to the isolated script and preserves failures.

The script builds services, checks ports, creates a run-specific schema, runs Playwright and cleans its data. Default console, gateway and upstream ports are 3100, 4100 and 4110; override E2E_UI_PORT, E2E_GW_PORT and E2E_UP_PORT. No path arguments enables full coverage checks.

`make testdata` clears the configured shared xhub target and Redis DB 1, seeds through a temporary gateway and retains the baseline. Use dedicated test data. `make e2e` reuses running services and may call paid providers; `make e2e-list` describes prerequisites and costs.

E2E_RUN_DIR contains results.json, junit.xml and scenario-coverage.json. E2E_BROWSER_REPORT_DIR selects an independent archive directory. Local fixtures do not establish real provider, identity-tenant or storage-account interoperability.
