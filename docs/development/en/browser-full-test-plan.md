# Automated browser regression

Use make e2e-offline for the isolated real console, gateway and PostgreSQL suite with local protocol fixtures. Select scenarios with bash scripts/e2e.sh scenarios/pages/docs.test.ts scenarios/docs. Install Chromium in frontend first. Default ports are 3100, 4100 and 4110; override E2E_UI_PORT, E2E_GW_PORT and E2E_UP_PORT.

make testdata clears the configured shared xhub target and Redis DB 1, seeds through a temporary gateway and retains the baseline. Use dedicated test data. make e2e reuses running services and may call paid providers. make e2e-list describes categories and prerequisites.

E2E_RUN_DIR contains results.json, junit.xml and scenario-coverage.json. E2E_BROWSER_REPORT_DIR selects a report archive. The isolated suite cleans its schema and scenario resources. Local fixtures do not establish real provider, identity-tenant or storage-account interoperability.
